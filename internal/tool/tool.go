package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

type Definition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Schema      map[string]any `json:"parameters"`
}

type Tool interface {
	Definition() Definition
	Execute(context.Context, json.RawMessage) (any, error)
}

type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

func NewRegistry(tools ...Tool) (*Registry, error) {
	registry := &Registry{tools: make(map[string]Tool, len(tools))}
	for _, item := range tools {
		if err := registry.Register(item); err != nil {
			return nil, err
		}
	}
	return registry, nil
}

func (r *Registry) Register(item Tool) error {
	definition := item.Definition()
	if definition.Name == "" {
		return fmt.Errorf("tool name is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tools[definition.Name]; exists {
		return fmt.Errorf("tool %q already registered", definition.Name)
	}
	r.tools[definition.Name] = item
	return nil
}

func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	item, ok := r.tools[name]
	return item, ok
}

func (r *Registry) Definitions(names []string) []Definition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	definitions := make([]Definition, 0, len(names))
	for _, name := range names {
		if item, ok := r.tools[name]; ok {
			definitions = append(definitions, item.Definition())
		}
	}
	return definitions
}
