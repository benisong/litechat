package service

import "context"

func BuildCharacterCardImportPlan(ctx context.Context, input []byte) (*JSONCharacterCardImportPlan, error) {
	parsed, err := NewCharacterCardInputRouter().Parse(ctx, input)
	if err != nil {
		return nil, err
	}
	normalized, err := NormalizeParsedCharacterCard(parsed)
	if err != nil {
		return nil, err
	}
	return &JSONCharacterCardImportPlan{
		CardVersion:        normalized.CardVersion,
		Character:          normalized.Character,
		Tags:               append([]string(nil), normalized.Tags...),
		PublicWorldBook:    normalized.PublicWorldBook,
		SchedulerWorldBook: normalized.SchedulerWorldBook,
		Extensions:         normalized.Extensions,
	}, nil
}
