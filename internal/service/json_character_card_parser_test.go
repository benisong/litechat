package service

import (
	"context"
	"encoding/json"
	"testing"
)

func TestJSONCharacterCardParserParsesEmbeddedWorldbook(t *testing.T) {
	raw := []byte(`{
		"card_version": "1.0",
		"character": {
			"name": "重生之玄幻之旅",
			"pov": "second",
			"description": "你是青霄宗大师兄。",
			"personality": "你克制而可靠。",
			"scenario": "玄霜心髓尚未启封。",
			"first_message": "寒玉匣在你面前。"
		},
		"worldbook": {
			"id": "rebirth-main",
			"name": "主世界书",
			"version": "1.0",
			"global_enabled": true,
			"main_entries": [{
				"id": "rule-1",
				"title": "基础规则",
				"content": "力量差距产生真实后果。",
				"enabled": true,
				"constant": true,
				"user_visible": true,
				"scheduler_enabled": false,
				"injection_position": 0,
				"injection_depth": 4,
				"scan_depth": 0,
				"role": "system"
			}],
			"sub_entries": [{
				"id": "hidden-1",
				"title": "隐藏状态",
				"content": "只供调度器使用。",
				"enabled": true,
				"constant": false,
				"user_visible": false,
				"scheduler_enabled": true,
				"injection_position": 1,
				"injection_depth": 2,
				"scan_depth": 3,
				"role": "system"
			}]
		},
		"tags": ["修仙", "复杂剧情"]
	}`)

	card, err := NewJSONCharacterCardParser().Parse(context.Background(), raw)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if card.Character.Name != "重生之玄幻之旅" || card.Character.POV != "second" {
		t.Fatalf("unexpected character: %+v", card.Character)
	}
	if len(card.WorldBook.MainEntries) != 1 || len(card.WorldBook.SubEntries) != 1 {
		t.Fatalf("unexpected worldbook entries: %+v", card.WorldBook)
	}
	if card.WorldBook.SubEntries[0].UserVisible || !card.WorldBook.SubEntries[0].SchedulerEnabled {
		t.Fatalf("hidden scheduler entry flags were not preserved: %+v", card.WorldBook.SubEntries[0])
	}
	if card.WorldBook.SubEntries[0].InjectionPosition != 1 || card.WorldBook.SubEntries[0].InjectionDepth != 2 || card.WorldBook.SubEntries[0].ScanDepth != 3 {
		t.Fatalf("injection controls were not preserved: %+v", card.WorldBook.SubEntries[0])
	}
}

func TestJSONCharacterCardParserRejectsDuplicateWorldbookEntryIDs(t *testing.T) {
	raw := []byte(`{"card_version":"1.0","character":{"name":"x","pov":"second"},"worldbook":{"id":"w","version":"1.0","global_enabled":true,"main_entries":[{"id":"same"}],"sub_entries":[{"id":"same"}]}}`)
	_, err := NewJSONCharacterCardParser().Parse(context.Background(), raw)
	if err == nil {
		t.Fatal("expected duplicate worldbook entry IDs to fail")
	}
}

func TestParsedCharacterCardRetainsUnknownExtensions(t *testing.T) {
	raw := []byte(`{"card_version":"1.0","character":{"name":"x","pov":"second"},"worldbook":{"id":"w","version":"1.0","global_enabled":true},"extensions":{"source":"user-upload"}}`)
	card, err := NewJSONCharacterCardParser().Parse(context.Background(), raw)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if string(card.Extensions["source"]) != `"user-upload"` {
		t.Fatalf("extension was not retained: %s", card.Extensions["source"])
	}
	if !json.Valid(card.Extensions["source"]) {
		t.Fatal("extension is not valid JSON")
	}
}
