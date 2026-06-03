package main

import (
	"context"
	"flag"
	"fmt"
	"iter"
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
)

type executor struct {
	agentName string
}

var _ a2asrv.AgentExecutor = (*executor)(nil)

func (e *executor) Execute(ctx context.Context, execCtx *a2asrv.ExecutorContext) iter.Seq2[a2a.Event, error] {
	return func(yield func(a2a.Event, error) bool) {
		var userText string
		if execCtx.Message != nil {
			for _, part := range execCtx.Message.Parts {
				if textContent, ok := part.Content.(a2a.Text); ok {
					userText += string(textContent)
				}
			}
		}
		if strings.TrimSpace(userText) == "" {
			userText = "Hello"
		}

		response := fmt.Sprintf("🤖 [%s] 收到你的消息: \"%s\"。你好！我是运行在 Kubernetes 上的 A2A Agent。",
			e.agentName, strings.TrimSpace(userText))

		yield(a2a.NewMessage(a2a.MessageRoleAgent, a2a.NewTextPart(response)), nil)
	}
}

func (e *executor) Cancel(ctx context.Context, execCtx *a2asrv.ExecutorContext) iter.Seq2[a2a.Event, error] {
	return func(yield func(a2a.Event, error) bool) {}
}

func main() {
	port := flag.Int("port", 9001, "Port for the A2A JSONRPC server")
	name := flag.String("name", getEnv("AGENT_NAME", "Hello Agent"), "Agent display name")
	desc := flag.String("desc", getEnv("AGENT_DESC", "A simple demo A2A agent"), "Agent description")
	skillID := flag.String("skill-id", getEnv("AGENT_SKILL_ID", "hello"), "Primary skill ID")
	flag.Parse()

	invokeURL := fmt.Sprintf("http://0.0.0.0:%d/invoke", *port)

	agentCard := &a2a.AgentCard{
		Name:        *name,
		Description: *desc,
		SupportedInterfaces: []*a2a.AgentInterface{
			a2a.NewAgentInterface(invokeURL, a2a.TransportProtocolJSONRPC),
		},
		DefaultInputModes:  []string{"text"},
		DefaultOutputModes: []string{"text"},
		Capabilities: a2a.AgentCapabilities{
			Streaming: false,
		},
		Skills: []a2a.AgentSkill{
			{
				ID:          *skillID,
				Name:        *name,
				Description: *desc,
				Tags:        []string{"demo", "hello", "example"},
				Examples:    []string{"Hello!", "Who are you?", "Say hi"},
			},
		},
		Version: "1.0.0",
	}

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("Failed to bind port %d: %v", *port, err)
	}

	requestHandler := a2asrv.NewHandler(&executor{agentName: *name})

	mux := http.NewServeMux()
	mux.Handle("/invoke", a2asrv.NewJSONRPCHandler(requestHandler))
	mux.Handle(a2asrv.WellKnownAgentCardPath, a2asrv.NewStaticAgentCardHandler(agentCard))

	log.Printf("🚀 A2A Agent [%s] listening on port %d", *name, *port)
	log.Printf("   Agent Card: http://0.0.0.0:%d%s", *port, a2asrv.WellKnownAgentCardPath)
	log.Printf("   Invoke:     http://0.0.0.0:%d/invoke", *port)

	if err := http.Serve(listener, mux); err != nil {
		log.Fatalf("Server stopped: %v", err)
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
