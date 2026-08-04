package service

import "fmt"

type CharacterCardParserRegistry struct {
	parsers map[string]map[string]CharacterCardParser
}

func NewCharacterCardParserRegistry() *CharacterCardParserRegistry {
	registry := &CharacterCardParserRegistry{parsers: make(map[string]map[string]CharacterCardParser)}
	registry.Register("json-character-card", "1.0", NewJSONCharacterCardParser())
	return registry
}

func (r *CharacterCardParserRegistry) Register(format, version string, parser CharacterCardParser) {
	if r.parsers == nil {
		r.parsers = make(map[string]map[string]CharacterCardParser)
	}
	if r.parsers[format] == nil {
		r.parsers[format] = make(map[string]CharacterCardParser)
	}
	r.parsers[format][version] = parser
}

func (r *CharacterCardParserRegistry) Resolve(format, version string) (CharacterCardParser, error) {
	if r == nil || r.parsers == nil {
		return nil, fmt.Errorf("character card parser registry is empty")
	}
	versions := r.parsers[format]
	if versions == nil {
		return nil, fmt.Errorf("unsupported character card format: %s", format)
	}
	parser := versions[version]
	if parser == nil {
		return nil, fmt.Errorf("unsupported character card version: %s/%s", format, version)
	}
	return parser, nil
}
