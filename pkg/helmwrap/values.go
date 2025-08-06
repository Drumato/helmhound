package helmwrap

import (
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// ExtractValuePaths recursively extracts all possible paths from a YAML string.
// Time complexity: O(n) where n is the total number of nodes in the YAML structure
// Space complexity: O(n) for storing all paths + O(d) for recursion stack depth d
func ExtractValuePaths(valuesYaml string) ([]string, error) {
	var data map[string]interface{}
	if err := yaml.Unmarshal([]byte(valuesYaml), &data); err != nil {
		return nil, err
	}

	var paths []string
	extractPaths("", data, &paths)
	return paths, nil
}

func extractPaths(prefix string, data interface{}, paths *[]string) {
	switch v := data.(type) {
	case map[string]interface{}:
		for key, value := range v {
			fullPath := key
			if prefix != "" {
				fullPath = prefix + "." + key
			}
			*paths = append(*paths, fullPath)
			extractPaths(fullPath, value, paths)
		}
	case []interface{}:
		for i, item := range v {
			indexPath := prefix + "[" + string(rune(i+'0')) + "]"
			*paths = append(*paths, indexPath)
			extractPaths(indexPath, item, paths)
		}
	}
}

// FilterValuesByPrefix filters a slice of strings to only include those with the given prefix.
// Time complexity: O(n*m) where n is the number of values and m is the average length of prefix
// Space complexity: O(k) where k is the number of matching values
func FilterValuesByPrefix(values []string, prefix string) []string {
	if prefix == "" {
		return values
	}

	var filtered []string
	for _, value := range values {
		if strings.HasPrefix(value, prefix) {
			filtered = append(filtered, value)
		}
	}
	return filtered
}

// ValueReference represents a reference to a value found in a template file.
// It contains information about where the reference was found and its context.
type ValueReference struct {
	File         string
	Line         int
	Content      string
	FullLine     string
	ManifestPath string // Path in the manifest where this value is used (e.g., ".spec.replicas")
}

// SearchValueInTemplates searches for all references to a specified value path in template files.
// It looks for various Helm template formats including {{ .Values.path }}, {{- .Values.path }}, etc.
// Returns a slice of ValueReference structs containing file, line, and match information.
// Time complexity: O(n*m) where n is total lines across all templates and m is average line length
// Space complexity: O(k) where k is the number of matching references
func SearchValueInTemplates(templates map[string]string, valuePath string) []ValueReference {
	var references []ValueReference

	// Create patterns to search for different value reference formats
	patterns := createSearchPatterns(valuePath)

	for fileName, content := range templates {
		lines := strings.Split(content, "\n")
		for lineNumber, line := range lines {
			// Track matches to avoid duplicates
			matchedPositions := make(map[int]bool)

			for _, pattern := range patterns {
				matches := pattern.FindAllStringIndex(line, -1)
				for _, match := range matches {
					// Check if this position was already matched
					if !matchedPositions[match[0]] {
						matchedPositions[match[0]] = true
						matchText := line[match[0]:match[1]]
						manifestPath := extractManifestPath(lines, lineNumber)
						references = append(references, ValueReference{
							File:         fileName,
							Line:         lineNumber + 1, // 1-indexed
							Content:      matchText,
							FullLine:     strings.TrimSpace(line),
							ManifestPath: manifestPath,
						})
					}
				}
			}
		}
	}

	return references
}

// createSearchPatterns creates regex patterns to match various Helm template value reference formats.
// It handles different template syntaxes including standard, left-trim, right-trim, and both-trim formats.
func createSearchPatterns(valuePath string) []*regexp.Regexp {
	// Escape dots in the value path for regex
	escapedPath := regexp.QuoteMeta(valuePath)

	var patterns []*regexp.Regexp

	// Simplified pattern: just match .Values.path.to.value with word boundary
	// This approach works better for complex template conditions
	pattern := regexp.MustCompile(`\.Values\.` + escapedPath + `\b`)
	patterns = append(patterns, pattern)

	return patterns
}

// extractManifestPath analyzes the template context to determine the manifest field path
// where the value is being used. It looks backward from the current line to find the
// YAML key that contains the value reference.
func extractManifestPath(lines []string, currentLineIndex int) string {
	if currentLineIndex >= len(lines) {
		return ""
	}

	currentLine := strings.TrimSpace(lines[currentLineIndex])

	// If the current line contains a YAML key-value pair, extract the key
	if strings.Contains(currentLine, ":") {
		parts := strings.SplitN(currentLine, ":", 2)
		if len(parts) >= 2 {
			key := strings.TrimSpace(parts[0])
			// Remove any YAML template markers
			key = strings.TrimPrefix(key, "{{-")
			key = strings.TrimSuffix(key, "-}}")
			key = strings.TrimPrefix(key, "{{")
			key = strings.TrimSuffix(key, "}}")
			key = strings.TrimSpace(key)

			// If this is a quoted key, remove quotes
			if (strings.HasPrefix(key, "\"") && strings.HasSuffix(key, "\"")) ||
				(strings.HasPrefix(key, "'") && strings.HasSuffix(key, "'")) {
				key = key[1 : len(key)-1]
			}

			if key != "" && !strings.Contains(key, ".Values") {
				// Build the path by looking at parent keys
				pathParts := []string{key}
				currentIndent := getIndentLevel(lines[currentLineIndex])

				// Look backward to find parent keys with less indentation
				for i := currentLineIndex - 1; i >= 0; i-- {
					line := lines[i]
					trimmedLine := strings.TrimSpace(line)

					// Skip empty lines and comments
					if trimmedLine == "" || strings.HasPrefix(trimmedLine, "#") {
						continue
					}

					lineIndent := getIndentLevel(line)

					// If this line has less indentation and contains a key, it's a parent
					if lineIndent < currentIndent && strings.Contains(trimmedLine, ":") {
						parts := strings.SplitN(trimmedLine, ":", 2)
						if len(parts) >= 2 {
							parentKey := strings.TrimSpace(parts[0])

							// Skip template control structures
							if strings.Contains(parentKey, "if") ||
								strings.Contains(parentKey, "range") ||
								strings.Contains(parentKey, "with") ||
								strings.Contains(parentKey, "define") ||
								strings.Contains(parentKey, "template") {
								continue
							}

							// Remove quotes if present
							if (strings.HasPrefix(parentKey, "\"") && strings.HasSuffix(parentKey, "\"")) ||
								(strings.HasPrefix(parentKey, "'") && strings.HasSuffix(parentKey, "'")) {
								parentKey = parentKey[1 : len(parentKey)-1]
							}

							if parentKey != "" && !strings.Contains(parentKey, ".Values") {
								pathParts = append([]string{parentKey}, pathParts...)
								currentIndent = lineIndent
							}
						}
					}

					// Stop if we reach the root level (kind:, apiVersion:, etc.)
					if lineIndent == 0 && (strings.HasPrefix(trimmedLine, "kind:") ||
						strings.HasPrefix(trimmedLine, "apiVersion:") ||
						strings.HasPrefix(trimmedLine, "metadata:")) {
						break
					}
				}

				if len(pathParts) > 0 {
					return "." + strings.Join(pathParts, ".")
				}
			}
		}
	}

	return ""
}

// getIndentLevel returns the indentation level of a line (number of leading spaces)
func getIndentLevel(line string) int {
	count := 0
	for _, char := range line {
		if char == ' ' {
			count++
		} else if char == '\t' {
			count += 2 // Count tabs as 2 spaces
		} else {
			break
		}
	}
	return count
}

// FormatValueReferences formats value references for display according to requirements.
// Groups references by file and displays them in the format: === filename === followed by line numbers and content.
// Returns a user-friendly message if no references are found.
func FormatValueReferences(references []ValueReference) string {
	if len(references) == 0 {
		return "No references found for the specified value."
	}

	// Group references by file
	fileGroups := make(map[string][]ValueReference)
	for _, ref := range references {
		fileGroups[ref.File] = append(fileGroups[ref.File], ref)
	}

	var result strings.Builder
	for fileName, refs := range fileGroups {
		result.WriteString(fmt.Sprintf("=== %s ===\n", fileName))
		for _, ref := range refs {
			manifestInfo := ref.ManifestPath
			if manifestInfo == "" {
				manifestInfo = ref.Content
			}
			result.WriteString(fmt.Sprintf("%dL: %s\n", ref.Line, manifestInfo))
		}
	}

	return result.String()
}
