package statusbar

import (
	"regexp"
	"strings"
)

const Marker = "【状态栏】"

var (
	fenceOnlyLine = regexp.MustCompile("(?m)^[ \\t]*(?:```[[:alnum:]_-]*|''')[ \\t]*(?:\\r?\\n|$)")
	fenceLanguage = regexp.MustCompile(`^[[:alnum:]_-]+$`)
)

// Split separates the final status panel from an assistant reply.
// The marker is searched from the end so quoted mentions earlier in the reply stay in the body.
func Split(content string) (body, panel string) {
	markerIndex := strings.LastIndex(content, Marker)
	if markerIndex < 0 {
		return content, ""
	}

	lineStart := strings.LastIndex(content[:markerIndex], "\n") + 1
	panelStart := lineStart

	// Include a standalone opening fence immediately above the marker.
	prefixWithoutNewlines := strings.TrimRight(content[:lineStart], "\r\n")
	if prefixWithoutNewlines != "" {
		previousLineStart := strings.LastIndex(prefixWithoutNewlines, "\n") + 1
		previousLine := strings.TrimSpace(prefixWithoutNewlines[previousLineStart:])
		if isFence(previousLine) {
			panelStart = previousLineStart
		}
	}

	body = strings.TrimRight(content[:panelStart], " \t\r\n")
	panel = fenceOnlyLine.ReplaceAllString(content[panelStart:], "")
	panel = strings.TrimSpace(panel)
	if panel == "" {
		return content, ""
	}
	return body, panel
}

func isFence(line string) bool {
	if line == "'''" {
		return true
	}
	if !strings.HasPrefix(line, "```") {
		return false
	}
	language := strings.TrimSpace(strings.TrimPrefix(line, "```"))
	return language == "" || fenceLanguage.MatchString(language)
}
