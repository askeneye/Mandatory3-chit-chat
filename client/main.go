package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	pb "Mandatory3_chitchat/grpc"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	conn, err := grpc.Dial("localhost:50052", grpc.WithTransportCredentials(insecure.NewCredentials()))
	// For someone else to connect we would have to replace localhost (above) with the server LAN IP address.
	// We would still have to be on the same network for this to work.
	// conn, err := grpc.Dial("10.26.58.14:50052", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewChatServiceClient(conn)

	// Create bidirectional stream
	stream, err := client.Chat(context.Background())
	if err != nil {
		log.Fatalf("Error creating stream: %v", err)
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter your name: ")
	name, _ := reader.ReadString('\n')
	id := int32(time.Now().UnixNano() % 10000)

	// Initial JOIN message
	joinMsg := &pb.JoinMsgLeave{
		Id:         id,
		ClientName: name,
		Type:       pb.EventType_JOIN,
		Timestamp:  0,
		Msg:        fmt.Sprintf("%s joined the chat", name),
	}

	if err := stream.Send(joinMsg); err != nil {
		log.Fatalf("Failed to send JOIN: %v", err)
	}
	log.Printf("[Client %s] Sent JOIN", name)

	var wg sync.WaitGroup
	wg.Add(2)
	done := make(chan struct{})

	// =====================
	// === Go Routine 1 ====
	// == Receive Messages =
	// =====================
	go func() {
		defer wg.Done()
		for {
			select {
			case <-done:
				log.Printf("[Client %s] Stopping receive loop", name)
				return
			default:
				in, err := stream.Recv()
				if err != nil {
					log.Printf("[Client %s] Stream closed: %v", name, err)
					return
				}
				fmt.Printf("\n[Server @ L=%d] %s: %s\n%s: ", in.Timestamp, in.SenderName, in.MsgStream, name)
			}
		}
	}()

	// =====================
	// === Go Routine 2 ====
	// === Send Messages ===
	// =====================
	go func() {
		defer wg.Done()
		reader := bufio.NewReader(os.Stdin)
		var lamport int64 = 0

		for {
			fmt.Printf("%s: ", name)
			text, _ := reader.ReadString('\n')
			text = strings.TrimSpace(text)

			if text == "exit" {
				lamport++
				stream.Send(&pb.JoinMsgLeave{
					Id:         id,
					ClientName: name,
					Type:       pb.EventType_LEAVE,
					Timestamp:  lamport,
					Msg:        fmt.Sprintf("%s left the chat", name),
				})
				fmt.Println("Leaving chat...")
				close(done)
				stream.CloseSend()
				return
			}

			lamport++
			msg := &pb.JoinMsgLeave{
				Id:         id,
				ClientName: name,
				Type:       pb.EventType_MESSAGE,
				Timestamp:  lamport,
				Msg:        text,
			}
			stream.Send(msg)
		}
	}()

	wg.Wait()
}
