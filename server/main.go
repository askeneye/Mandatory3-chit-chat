package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"

	pb "Mandatory3_chitchat/grpc"

	"google.golang.org/grpc"
)

// =====================
// ==== Structures =====
// =====================

// The Client struct is one connected chat user
type Client struct {
	id     int32
	name   string
	stream pb.ChatService_ChatServer
	active bool
	err    chan error
}

// ChatServiceServer implement of the gRPC ChatService
type ChatServiceServer struct {
	pb.UnimplementedChatServiceServer
	mu      sync.Mutex
	clients map[int32]*Client
	clock   int64 // Lamport clock
}

// =====================
// ==== Core Logic =====
// =====================

// Chat() method handles the bidirectional stream with each client
func (s *ChatServiceServer) Chat(stream pb.ChatService_ChatServer) error {
	// Read first message from the client
	msg, err := stream.Recv()
	if err != nil {
		log.Printf("[Server] Failed initial recv: %v", err)
		return err
	}

	// Check if EventType is actually a JOIN request
	if msg.Type != pb.EventType_JOIN {
		log.Printf("[Server] First message from client %s was not JOIN (got %v)", msg.ClientName, msg.Type)
		return fmt.Errorf("expected JOIN as first message")
	}

	s.mu.Lock()
	// Check to ensure no duplicate clients
	if _, exists := s.clients[msg.Id]; exists {
		s.mu.Unlock()
		log.Printf("[Server] Duplicate ID %d from client %s", msg.Id, msg.ClientName)
		return fmt.Errorf("client ID already connected")
	}

	cleanName := strings.TrimSpace(msg.ClientName)

	client := &Client{
		id:     msg.Id,
		name:   cleanName,
		stream: stream,
		active: true,
		err:    make(chan error, 1),
	}
	s.clients[client.id] = client

	// Lamport clock update
	s.clock = max(s.clock, msg.Timestamp) + 1
	joinLamport := s.clock
	s.mu.Unlock()

	log.Printf("[Server] JOIN: %s (ID=%d) at L=%d", client.name, client.id, joinLamport)

	// Broadcast join message
	s.broadcast(&pb.ServerMessage{
		SenderId:   client.id,
		SenderName: client.name,
		MsgStream:  fmt.Sprintf("Participant %s joined Chit Chat at logical time L=%d", client.name, joinLamport),
		Type:       pb.EventType_JOIN,
		Timestamp:  joinLamport,
	})

	// Start goroutine and receive messages from this client
	go func() {
		for {
			msg, err := stream.Recv()
			if err == io.EOF {
				// client closed the connection
				break
			}
			if err != nil {
				client.err <- err
				break
			}

			if msg.Type == pb.EventType_LEAVE {
				s.mu.Lock()
				s.clock = max(s.clock, msg.Timestamp) + 1
				currentLamport := s.clock
				s.mu.Unlock()

				// Broadcast to everyone that this client left
				s.broadcast(&pb.ServerMessage{
					SenderId:   client.id,
					SenderName: client.name,
					MsgStream:  fmt.Sprintf("Participant %s left Chit Chat at logical time L=%d", client.name, currentLamport),
					Type:       pb.EventType_LEAVE,
					Timestamp:  currentLamport,
				})

				// Remove from server list
				s.removeClient(client)

				// Notify the outer Chat() function to end cleanly
				client.err <- io.EOF
				return
			}

			// Handle message size of max 128 characters
			cleanMsg := strings.TrimSpace(msg.Msg)
			if len(cleanMsg) > 128 {
				s.mu.Lock()
				s.clock = max(s.clock, msg.Timestamp) + 1
				currentLamport := s.clock
				s.mu.Unlock()

				log.Printf("[Server] Rejecting oversize message from %s (len=%d) at L=%d", client.name, len(cleanMsg), currentLamport)

				// Letting the sender know that their message was rejected if message too long
				_ = client.stream.Send(&pb.ServerMessage{
					SenderId:   0,
					SenderName: "Server",
					MsgStream:  fmt.Sprintf("Message too long (%d chars). Limit is 128.", len(cleanMsg)),
					Type:       pb.EventType_MESSAGE,
					Timestamp:  currentLamport,
				})

				continue
			}

			// Update Lamport clock
			s.mu.Lock()
			s.clock = max(s.clock, msg.Timestamp) + 1
			currentLamport := s.clock
			s.mu.Unlock()

			// Log and broadcast
			log.Printf("[Server] Message from %s: %s (L=%d)", msg.ClientName, msg.Msg, currentLamport)

			s.broadcast(&pb.ServerMessage{
				SenderId:   client.id,
				SenderName: client.name,
				MsgStream:  msg.Msg,
				Type:       pb.EventType_MESSAGE,
				Timestamp:  currentLamport,
			})
		}

		// Handle client leaving
		s.removeClient(client)
		client.err <- io.EOF // Signal main Chat() to exit cleanly
	}()

	// Block until this client's error channel receives something
	return <-client.err
}

// Broadcast message to all connected clients
func (s *ChatServiceServer) broadcast(msg *pb.ServerMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, c := range s.clients {
		if c.active {
			go func(cl *Client) {
				if err := cl.stream.Send(msg); err != nil {
					log.Printf("[Server] Error sending to %s: %v", cl.name, err)
					cl.active = false
				}
			}(c)
		}
	}
}

// Remove client and broadcast leave message
func (s *ChatServiceServer) removeClient(client *Client) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.clock++
	leaveLamport := s.clock
	delete(s.clients, client.id)

	log.Printf("[Server] LEAVE: %s (ID=%d) at L=%d", client.name, client.id, leaveLamport)

	s.broadcast(&pb.ServerMessage{
		SenderId:   client.id,
		SenderName: client.name,
		MsgStream:  fmt.Sprintf("%s left Chit Chat", client.name),
		Type:       pb.EventType_LEAVE,
		Timestamp:  leaveLamport,
	})
}

// =====================
// ==== Main Block =====
// =====================

func main() {
	port := ":50052"
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("Failed to listen on port %s: %v", port, err)
	}

	server := &ChatServiceServer{
		clients: make(map[int32]*Client),
		clock:   0,
	}

	grpcServer := grpc.NewServer()
	pb.RegisterChatServiceServer(grpcServer, server)

	log.Printf("[Server] Chit Chat service running on %s", port)

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}

// =====================
// == Helper function ==
// ======== for ========
// === Lamport Clock ===
// =====================

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
