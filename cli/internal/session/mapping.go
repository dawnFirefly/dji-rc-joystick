package session

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	datasettings "github.com/voluminor/dji-rc-joystick/mod/data/settings"
	"github.com/voluminor/dji-rc-joystick/mod/rc"
	targetsettings "github.com/voluminor/dji-rc-joystick/target/settings"
	"gopkg.in/yaml.v3"
)

// // // // // // // // // //

const maxConfigSize = 1 << 20

func parseJSONMapping(data []byte) (*targetsettings.ConfigObj, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	token, err := decoder.Token()
	if err != nil {
		return nil, fmt.Errorf("parse JSON config: %w", err)
	}
	if delimiter, ok := token.(json.Delim); !ok || delimiter != '{' {
		return nil, errors.New("config root must be an object")
	}

	var mapping json.RawMessage
	found := false
	for decoder.More() {
		token, tokenErr := decoder.Token()
		if tokenErr != nil {
			return nil, fmt.Errorf("parse JSON config key: %w", tokenErr)
		}
		key, ok := token.(string)
		if !ok {
			return nil, errors.New("config object contains a non-string key")
		}
		var value json.RawMessage
		if decodeErr := decoder.Decode(&value); decodeErr != nil {
			return nil, fmt.Errorf("parse JSON config field %q: %w", key, decodeErr)
		}
		if key == "mapping" {
			if found {
				return nil, errors.New("config contains duplicate mapping sections")
			}
			found = true
			mapping = append(mapping[:0], value...)
		}
	}
	if _, err = decoder.Token(); err != nil {
		return nil, fmt.Errorf("parse JSON config: %w", err)
	}
	var trailing any
	if trailingErr := decoder.Decode(&trailing); trailingErr != io.EOF {
		if trailingErr == nil {
			return nil, errors.New("config contains data after the root object")
		}
		return nil, fmt.Errorf("parse JSON config trailer: %w", trailingErr)
	}
	if !found {
		return nil, errors.New("config has no root mapping section")
	}

	format, _ := targetsettings.FormatByName("json")
	synthetic := append([]byte(`{"mapping":`), mapping...)
	synthetic = append(synthetic, '}')
	config, err := format.Parse(synthetic)
	if err != nil {
		return nil, fmt.Errorf("parse mapping: %w", err)
	}
	return config, nil
}

func parseYAMLMapping(data []byte) (*targetsettings.ConfigObj, error) {
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("parse YAML config: %w", err)
	}
	if document.Kind != yaml.DocumentNode || len(document.Content) == 0 {
		return nil, errors.New("config document is empty")
	}
	root := document.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, errors.New("config root must be a mapping")
	}

	var mapping *yaml.Node
	for index := 0; index+1 < len(root.Content); index += 2 {
		key := root.Content[index]
		if key.Value != "mapping" {
			continue
		}
		if mapping != nil {
			return nil, errors.New("config contains duplicate mapping sections")
		}
		mapping = root.Content[index+1]
	}
	if mapping == nil {
		return nil, errors.New("config has no root mapping section")
	}

	syntheticRoot := &yaml.Node{
		Kind: yaml.MappingNode,
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: "mapping"},
			mapping,
		},
	}
	synthetic := &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{syntheticRoot}}
	rendered, err := yaml.Marshal(synthetic)
	if err != nil {
		return nil, fmt.Errorf("prepare mapping: %w", err)
	}
	format, _ := targetsettings.FormatByName("yaml")
	config, err := format.Parse(rendered)
	if err != nil {
		return nil, fmt.Errorf("parse mapping: %w", err)
	}
	return config, nil
}

func readConfig(path string) ([]byte, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, "", fmt.Errorf("open config: %w", err)
	}
	defer func() { _ = file.Close() }()

	info, err := file.Stat()
	if err != nil {
		return nil, "", fmt.Errorf("stat config: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, "", errors.New("config is not a regular file")
	}
	if info.Size() > maxConfigSize {
		return nil, "", fmt.Errorf("config exceeds the %d-byte limit", maxConfigSize)
	}

	data, err := io.ReadAll(io.LimitReader(file, maxConfigSize+1))
	if err != nil {
		return nil, "", fmt.Errorf("read config: %w", err)
	}
	if len(data) > maxConfigSize {
		return nil, "", fmt.Errorf("config exceeds the %d-byte limit", maxConfigSize)
	}
	extension := strings.ToLower(filepath.Ext(path))
	return data, extension, nil
}

// DefaultMapping returns a fresh copy of the mapping compiled into the target.
func DefaultMapping() (rc.MappingObj, error) {
	mapping := datasettings.ControlMapping(targetsettings.New().Mapping)
	if err := mapping.Validate(); err != nil {
		return rc.MappingObj{}, fmt.Errorf("validate built-in mapping: %w", err)
	}
	return mapping, nil
}

// LoadMapping reads and strictly validates only the root mapping section.
func LoadMapping(path string) (rc.MappingObj, error) {
	data, extension, err := readConfig(path)
	if err != nil {
		return rc.MappingObj{}, err
	}

	var config *targetsettings.ConfigObj
	switch extension {
	case ".json":
		config, err = parseJSONMapping(data)
	case ".yaml", ".yml":
		config, err = parseYAMLMapping(data)
	default:
		return rc.MappingObj{}, fmt.Errorf("unsupported config extension %q (allowed: .json|.yaml|.yml)", extension)
	}
	if err != nil {
		return rc.MappingObj{}, err
	}

	mapping := datasettings.ControlMapping(config.Mapping)
	if err = mapping.Validate(); err != nil {
		return rc.MappingObj{}, fmt.Errorf("validate mapping: %w", err)
	}
	return mapping, nil
}
