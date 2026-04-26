package policy

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/thkx/agentkernel/types"
	"gopkg.in/yaml.v3"
)

// PolicyLoader loads policy configurations from files
type PolicyLoader struct {
	// Optional: validator to validate config after loading
	validator *PolicyValidator
	bus       types.EventBus
}

// NewPolicyLoader creates a new policy loader
func NewPolicyLoader() *PolicyLoader {
	return &PolicyLoader{}
}

func (pl *PolicyLoader) WithBus(bus types.EventBus) *PolicyLoader {
	pl.bus = bus
	return pl
}

// LoadFromJSON loads a policy configuration from a JSON file
func (pl *PolicyLoader) LoadFromJSON(filename string) (*PolicyConfig, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read JSON file %s: %w", filename, err)
	}

	var config PolicyConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	if err := pl.validateLoadedConfig(&config); err != nil {
		publishPolicyEvent(pl.bus, "policy.config_load_failed", config.Name, "json_file", false, len(config.Nodes), err, 0, 1, map[string]any{"filename": filename})
		return nil, err
	}
	publishPolicyEvent(pl.bus, "policy.config_loaded", config.Name, "json_file", true, len(config.Nodes), nil, 0, 0, map[string]any{"filename": filename})

	return &config, nil
}

// LoadFromYAML loads a policy configuration from a YAML file
func (pl *PolicyLoader) LoadFromYAML(filename string) (*PolicyConfig, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read YAML file %s: %w", filename, err)
	}

	var config PolicyConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML: %w", err)
	}

	if err := pl.validateLoadedConfig(&config); err != nil {
		publishPolicyEvent(pl.bus, "policy.config_load_failed", config.Name, "yaml_file", false, len(config.Nodes), err, 0, 1, map[string]any{"filename": filename})
		return nil, err
	}
	publishPolicyEvent(pl.bus, "policy.config_loaded", config.Name, "yaml_file", true, len(config.Nodes), nil, 0, 0, map[string]any{"filename": filename})

	return &config, nil
}

// LoadFromJSONString loads a policy configuration from a JSON string
func (pl *PolicyLoader) LoadFromJSONString(jsonStr string) (*PolicyConfig, error) {
	var config PolicyConfig
	if err := json.Unmarshal([]byte(jsonStr), &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	if err := pl.validateLoadedConfig(&config); err != nil {
		publishPolicyEvent(pl.bus, "policy.config_load_failed", config.Name, "json_string", false, len(config.Nodes), err, 0, 1, nil)
		return nil, err
	}
	publishPolicyEvent(pl.bus, "policy.config_loaded", config.Name, "json_string", true, len(config.Nodes), nil, 0, 0, nil)

	return &config, nil
}

// LoadFromYAMLString loads a policy configuration from a YAML string
func (pl *PolicyLoader) LoadFromYAMLString(yamlStr string) (*PolicyConfig, error) {
	var config PolicyConfig
	if err := yaml.Unmarshal([]byte(yamlStr), &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML: %w", err)
	}

	if err := pl.validateLoadedConfig(&config); err != nil {
		publishPolicyEvent(pl.bus, "policy.config_load_failed", config.Name, "yaml_string", false, len(config.Nodes), err, 0, 1, nil)
		return nil, err
	}
	publishPolicyEvent(pl.bus, "policy.config_loaded", config.Name, "yaml_string", true, len(config.Nodes), nil, 0, 0, nil)

	return &config, nil
}

// SaveToJSON saves a policy configuration to a JSON file
func (pl *PolicyLoader) SaveToJSON(config *PolicyConfig, filename string) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config to JSON: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write JSON file %s: %w", filename, err)
	}

	return nil
}

// SaveToYAML saves a policy configuration to a YAML file
func (pl *PolicyLoader) SaveToYAML(config *PolicyConfig, filename string) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config to YAML: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write YAML file %s: %w", filename, err)
	}

	return nil
}

// validateLoadedConfig performs basic validation on loaded config
func (pl *PolicyLoader) validateLoadedConfig(config *PolicyConfig) error {
	if config.Start == "" {
		return fmt.Errorf("policy config: start node not specified")
	}

	if config.Name == "" {
		return fmt.Errorf("policy config: name not specified")
	}

	if len(config.Nodes) == 0 {
		return fmt.Errorf("policy config: no nodes defined")
	}

	// Validate start node exists
	if _, exists := config.Nodes[config.Start]; !exists {
		return fmt.Errorf("policy config: start node '%s' not found in nodes", config.Start)
	}

	// Validate all node IDs match their key in the map
	for nodeID, nodeCfg := range config.Nodes {
		if nodeCfg.ID == "" {
			nodeCfg.ID = nodeID
		}
		if nodeCfg.ID != nodeID {
			return fmt.Errorf("policy config: node ID mismatch - key '%s' vs node.id '%s'", nodeID, nodeCfg.ID)
		}

		// Validate capability is specified
		if nodeCfg.Capability == "" {
			return fmt.Errorf("policy config: node '%s' missing capability", nodeID)
		}

		// Validate timeout is positive if specified
		if nodeCfg.Config != nil && nodeCfg.Config.Timeout != nil && *nodeCfg.Config.Timeout < 0 {
			return fmt.Errorf("policy config: node '%s' timeout must be non-negative", nodeID)
		}

		// Validate rate limit is positive if specified
		if nodeCfg.Config != nil && nodeCfg.Config.RateLimit < 0 {
			return fmt.Errorf("policy config: node '%s' rate limit must be non-negative", nodeID)
		}

		// Validate priority is reasonable
		if nodeCfg.Config != nil && nodeCfg.Config.Priority < 0 {
			return fmt.Errorf("policy config: node '%s' priority must be non-negative", nodeID)
		}

		// Validate edges
		for _, edge := range nodeCfg.Edges {
			if edge.To == "" {
				return fmt.Errorf("policy config: node '%s' has edge with empty 'to' field", nodeID)
			}
			// Check if target node exists
			if _, exists := config.Nodes[edge.To]; !exists {
				return fmt.Errorf("policy config: node '%s' references non-existent node '%s'", nodeID, edge.To)
			}
		}
	}

	return nil
}
