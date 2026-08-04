package service

// FilterParsedWorldBookForUser returns the public, enabled part of a parsed book.
// It deliberately strips scheduler-only entries before the result reaches user-facing APIs.
func FilterParsedWorldBookForUser(book ParsedWorldBook) ParsedWorldBook {
	filtered := ParsedWorldBook{ID: book.ID, Name: book.Name, Version: book.Version, GlobalEnabled: book.GlobalEnabled}
	if !book.GlobalEnabled {
		return filtered
	}
	filtered.MainEntries = filterEntries(book.MainEntries, func(entry ParsedWorldBookEntry) bool {
		return entry.Enabled && entry.UserVisible
	})
	filtered.SubEntries = filterEntries(book.SubEntries, func(entry ParsedWorldBookEntry) bool {
		return entry.Enabled && entry.UserVisible
	})
	return filtered
}

// FilterParsedWorldBookForScheduler returns the enabled scheduler track.
// Effects are not executed here; this is only the source material for initialization
// and scheduler compilation.
func FilterParsedWorldBookForScheduler(book ParsedWorldBook) ParsedWorldBook {
	filtered := ParsedWorldBook{ID: book.ID, Name: book.Name, Version: book.Version, GlobalEnabled: book.GlobalEnabled}
	if !book.GlobalEnabled {
		return filtered
	}
	filtered.MainEntries = filterEntries(book.MainEntries, func(entry ParsedWorldBookEntry) bool {
		return entry.Enabled && entry.SchedulerEnabled
	})
	filtered.SubEntries = filterEntries(book.SubEntries, func(entry ParsedWorldBookEntry) bool {
		return entry.Enabled && entry.SchedulerEnabled
	})
	return filtered
}

func filterEntries(entries []ParsedWorldBookEntry, keep func(ParsedWorldBookEntry) bool) []ParsedWorldBookEntry {
	filtered := make([]ParsedWorldBookEntry, 0, len(entries))
	for _, entry := range entries {
		if keep(entry) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}
