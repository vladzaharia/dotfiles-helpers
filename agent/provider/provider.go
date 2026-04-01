package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	iexec "github.com/vladzaharia/dotfiles-helpers/internal/exec"
	"github.com/vladzaharia/dotfiles-helpers/internal/output"
)

type Status struct {
	Name      string
	Installed bool
	Version   string
	Detail    string
}

func DetectClaude() Status {
	s := Status{Name: "Claude Code"}
	path, err := iexec.FindBinary("claude")
	if err != nil {
		return s
	}
	s.Installed = true
	ver, err := iexec.Run(path, "--version")
	if err == nil {
		s.Version = strings.TrimSpace(ver)
		s.Detail = fmt.Sprintf("v%s", s.Version)
	}
	return s
}

func DetectCodex() Status {
	s := Status{Name: "Codex CLI"}
	path, err := iexec.FindBinary("codex")
	if err != nil {
		return s
	}
	s.Installed = true
	ver, err := iexec.Run(path, "--version")
	if err == nil {
		s.Version = strings.TrimSpace(ver)
		s.Detail = fmt.Sprintf("v%s", s.Version)
	}
	return s
}

func DetectLMStudio(url string) Status {
	s := Status{Name: "LM Studio"}
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url + "/v1/models")
	if err != nil {
		s.Detail = "not reachable"
		return s
	}
	defer resp.Body.Close()

	s.Installed = true
	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err == nil {
		s.Detail = fmt.Sprintf("%d models", len(result.Data))
	}
	return s
}

func DetectOllama() Status {
	s := Status{Name: "Ollama"}
	_, err := iexec.FindBinary("ollama")
	if err != nil {
		return s
	}
	s.Installed = true
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://localhost:11434/api/tags")
	if err != nil {
		s.Detail = "installed, not running"
		return s
	}
	defer resp.Body.Close()
	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err == nil {
		s.Detail = fmt.Sprintf("%d models", len(result.Models))
	}
	return s
}

func PrintStatus(statuses []Status) {
	for _, s := range statuses {
		if s.Installed {
			fmt.Println(output.StatusOK(s.Name, s.Detail))
		} else if s.Detail != "" {
			fmt.Println(output.StatusFail(s.Name, s.Detail))
		} else {
			fmt.Println(output.StatusNone(s.Name, "not installed"))
		}
	}
}
