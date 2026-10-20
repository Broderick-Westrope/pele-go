package oracle

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
)

// Node runs the same bundle in Node, under V8, for what goja gets wrong: it keeps lone
// surrogates where goja's trim, concatenation and JSON.parse replace them.
type Node struct {
	cmd *exec.Cmd
	in  io.WriteCloser
	out *bufio.Reader
}

// NodeOptions are the options for one render. Measurer names a measurer the driver script
// defines ("length"), or "" for Pele's default.
type NodeOptions struct {
	Options  map[string]any
	Measurer string
	Now      float64
}

// NewNode starts node with the driver script. It returns exec.ErrNotFound when node is not
// installed.
func NewNode() (*Node, error) {
	path, err := exec.LookPath("node")
	if err != nil {
		return nil, err
	}
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Join(filepath.Dir(file), "testdata")
	if _, err := os.Stat(filepath.Join(dir, "pele.global.js")); err != nil {
		return nil, err
	}
	cmd := exec.Command(path, filepath.Join(dir, "node-oracle.cjs"), filepath.Join(dir, "pele.global.js"))
	cmd.Stderr = os.Stderr
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start node: %w", err)
	}
	return &Node{cmd: cmd, in: in, out: bufio.NewReaderSize(out, 1<<20)}, nil
}

// Close stops node.
func (n *Node) Close() error {
	_ = n.in.Close()
	return n.cmd.Wait()
}

type nodeLink struct {
	ID       []uint16 `json:"id"`
	Href     []uint16 `json:"href"`
	Internal bool     `json:"internal"`
}

type nodeResponse struct {
	SVG         []uint16   `json:"svg"`
	Width       float64    `json:"width"`
	Height      float64    `json:"height"`
	Type        string     `json:"type"`
	MermaidType string     `json:"mermaidType"`
	Links       []nodeLink `json:"links"`
	Error       *struct {
		Code        string   `json:"code"`
		Message     []uint16 `json:"message"`
		Type        *string  `json:"type"`
		MermaidType *string  `json:"mermaidType"`
		Line        int      `json:"line"`
		Column      int      `json:"column"`
		Snippet     []uint16 `json:"snippet"`
	} `json:"error"`
	Thrown string `json:"thrown"`
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// Render calls pele.render in node.
func (n *Node) Render(src string, opts NodeOptions) (*Result, *Error, error) {
	req := map[string]any{"src": jsstr.UTF16(src), "options": opts.Options}
	if opts.Measurer != "" {
		req["measurer"] = opts.Measurer
	}
	if opts.Now != 0 {
		req["now"] = opts.Now
	}
	line, err := json.Marshal(req)
	if err != nil {
		return nil, nil, err
	}
	if _, err := n.in.Write(append(line, '\n')); err != nil {
		return nil, nil, fmt.Errorf("write to node: %w", err)
	}
	reply, err := n.out.ReadBytes('\n')
	if err != nil {
		return nil, nil, fmt.Errorf("read from node: %w", err)
	}
	var resp nodeResponse
	if err := json.Unmarshal(reply, &resp); err != nil {
		return nil, nil, fmt.Errorf("decode node reply: %w", err)
	}
	if resp.Thrown != "" {
		return nil, nil, fmt.Errorf("node threw: %s", resp.Thrown)
	}
	if e := resp.Error; e != nil {
		return nil, &Error{
			Code: e.Code, Message: jsstr.FromUTF16(e.Message), Type: deref(e.Type), MermaidType: deref(e.MermaidType),
			Line: e.Line, Column: e.Column, Snippet: jsstr.FromUTF16(e.Snippet),
		}, nil
	}
	res := &Result{
		SVG: jsstr.FromUTF16(resp.SVG), Width: resp.Width, Height: resp.Height,
		Type: resp.Type, MermaidType: resp.MermaidType,
	}
	for _, l := range resp.Links {
		res.Links = append(res.Links, Link{ID: jsstr.FromUTF16(l.ID), Href: jsstr.FromUTF16(l.Href), Internal: l.Internal})
	}
	return res, nil, nil
}
