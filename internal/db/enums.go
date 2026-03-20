package db

// Enum mappings from integer columns to API strings.
// NOTE: Values are based on Rails enums and may need adjustment if those change.

var partOfSpeechMap = map[int]string{
	0: "noun",
	1: "verb",
	2: "adjective",
	3: "adverb",
	4: "pronoun",
	5: "preposition",
	6: "conjunction",
	7: "interjection",
	8: "article",
	9: "modifier",
}

func PartOfSpeechFromInt(v int) string {
	if s, ok := partOfSpeechMap[v]; ok {
		return s
	}
	return "noun"
}

// Pictos visibility: map DB integer to conceptual values.
// 0 is assumed to mean "everybody" (public).
func IsVisibilityEverybody(v int) bool {
	return v == 0
}

// Symbolset status mapping (Rails enum):
//   published: 0
//   draft:     1
//   ingesting: 2
func SymbolsetStatusFromInt(v int) string {
	switch v {
	case 0:
		return "published"
	case 2:
		return "ingesting"
	default:
		return "draft"
	}
}

func IsSymbolsetPublished(v int) bool {
	return SymbolsetStatusFromInt(v) == "published"
}

// Coding framework structure mapping: 0=linked_data, 1=legacy (assumed).
func CodingFrameworkStructureFromInt(v int) string {
	switch v {
	case 1:
		return "legacy"
	default:
		return "linked_data"
	}
}

