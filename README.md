----------------------------------------------------
Chit-Chat — Distributed gRPC Chat with Lamport Clocks
----------------------------------------------------

A simple distributed chat system built with Go and gRPC, demonstrating bidirectional streaming RPCs and Lamport logical clocks to maintain consistent event ordering across multiple clients.


----------------------------------------------------
Quick Start
----------------------------------------------------

1. Clone the repository
   git clone https://github.com/askeneye/Mandatory3-chit-chat.git
   cd Mandatory3-chit-chat

2. Generate gRPC code
   Make sure you have protoc and Go gRPC plugins installed:

   go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
   go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

   Then generate the protobuf files:
   protoc --go_out=. --go-grpc_out=. --go_opt=paths=source_relative --go-grpc_opt=paths=source_relative grpc/chat.proto


----------------------------------------------------
Run the Application
----------------------------------------------------

Start the server:
   go run ./server

You should see:
   [Server] Chit Chat service running on :50052

Start one or more clients:
   go run ./client

Enter your name when prompted, then start chatting!
Type "exit" to leave the chat.


----------------------------------------------------
Network Setup
----------------------------------------------------

By default, the client connects to:
   "localhost:50052"

To allow other computers on the same network to connect, replace "localhost" with your LAN IPv4 address, for example:
   "192.168.x.x:50052"


----------------------------------------------------
Lamport Clock
----------------------------------------------------

Each join, message, and leave event is timestamped using a Lamport logical clock maintained by the server.

Formula:
   L = max(L_server, L_client) + 1



----------------------------------------------------
Example Output
----------------------------------------------------

Server:
   [Server] Chit Chat service running on :50052
   [Server] JOIN: Alex (ID=3300) at L=1
   [Server] Message from Alex: Hello World! (L=2)
   [Server] JOIN: Aske (ID=4800) at L=3
   [Server] Message from Aske: Hello Alex (L=4)
   [Server] LEAVE: Aske (ID=4800) at L=8

Client:
   Enter your name: Aske
   Aske: Hello Alex
   [Server @ L=4] Alex: Hello World!
   Aske: exit
   Leaving chat...
