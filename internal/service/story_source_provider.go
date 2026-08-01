package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"litechat/internal/store"
	"sort"
	"strings"
)

type StoryCompileSource struct {
	Text        string
	VersionHash string
}

type StorySourceProvider interface {
	Load(ctx context.Context, userID, characterID string) (StoryCompileSource, error)
}

type WorldBookStorySourceProvider struct {
	worldBookStore *store.WorldBookStore
}

func NewWorldBookStorySourceProvider(worldBookStore *store.WorldBookStore) *WorldBookStorySourceProvider {
	return &WorldBookStorySourceProvider{worldBookStore: worldBookStore}
}

func (p *WorldBookStorySourceProvider) Load(ctx context.Context, userID, characterID string) (StoryCompileSource, error) {
	if err := ctx.Err(); err != nil {
		return StoryCompileSource{}, err
	}
	if p == nil || p.worldBookStore == nil {
		return StoryCompileSource{}, fmt.Errorf("story source provider is not configured")
	}
	books, err := p.worldBookStore.List(userID)
	if err != nil {
		return StoryCompileSource{}, err
	}
	sort.Slice(books, func(i, j int) bool {
		if books[i].Name != books[j].Name {
			return books[i].Name < books[j].Name
		}
		return books[i].ID < books[j].ID
	})
	var builder strings.Builder
	hash := sha256.New()
	for _, book := range books {
		if book.RuntimeMode != "compile_only" || (book.CharacterID != "" && book.CharacterID != characterID) {
			continue
		}
		entries, err := p.worldBookStore.ListEntries(book.ID, userID)
		if err != nil {
			return StoryCompileSource{}, err
		}
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].Order != entries[j].Order {
				return entries[i].Order < entries[j].Order
			}
			return entries[i].ID < entries[j].ID
		})
		for _, entry := range entries {
			if !entry.Enabled {
				continue
			}
			canonical := fmt.Sprintf("book=%s\nname=%s\nkey=%s\nsecondary=%s\norder=%d\ncontent=%s\n", book.ID, book.Name, entry.Keys, entry.SecondaryKeys, entry.Order, entry.Content)
			_, _ = hash.Write([]byte(canonical))
			builder.WriteString("## ")
			builder.WriteString(book.Name)
			builder.WriteString(" / ")
			builder.WriteString(entry.Keys)
			builder.WriteString("\n")
			builder.WriteString(entry.Content)
			builder.WriteString("\n\n")
		}
	}
	return StoryCompileSource{Text: builder.String(), VersionHash: hex.EncodeToString(hash.Sum(nil))}, nil
}
