package apple

import (
	"regexp"
	"sort"
	"strings"
)

// matching.go holds the title matching that decides which directory entry is
// the show someone means. Podcast titles are hostile input: the same show is
// "Turncloaks" in the directory and "Turncloaks - D&D5E Dark Fantasy Actual
// Play" on disk, and Apple's search ranks by popularity, so a small show can be
// absent from an exact-phrase query entirely. The answer is several queries
// plus a real similarity score, not one query and a threshold.

// MatchFloor is the confidence below which a candidate is not worth showing.
// Set from the cases that must survive: a long disk title against a short
// directory name scores near 1.0, while an unrelated show in the same genre
// scores under 0.2.
const MatchFloor = 0.42

// titleNoise are the words that describe the format rather than the show. They
// are dropped before comparison, because "Dark Fantasy Actual Play" appears on
// half of all actual-play shows and matches everything.
var titleNoise = map[string]struct{}{
	"a": {}, "an": {}, "and": {}, "the": {}, "of": {}, "or": {}, "in": {},
	"podcast": {}, "podcasts": {}, "show": {}, "series": {}, "official": {},
	"actual": {}, "play": {}, "actualplay": {}, "rpg": {}, "ttrpg": {},
	"dnd": {}, "dd": {}, "dungeons": {}, "dragons": {}, "5e": {}, "5th": {},
	"edition": {}, "dark": {}, "fantasy": {}, "audio": {}, "audiodrama": {},
	"adventure": {}, "campaign": {}, "live": {}, "liveplay": {}, "let": {},
	"lets": {}, "presents": {}, "network": {}, "d": {},
}

// editionTokenPattern catches "d5e", "5e", "3e", "e5" style edition markers that
// survive tokenising "D&D5E".
var editionTokenPattern = regexp.MustCompile(`^(d?\d+e|e\d*)$`)

var (
	nonAlnumPattern = regexp.MustCompile(`[^a-z0-9]+`)
	variantSplitter = []string{" - ", " – ", " — ", " | ", ": ", " —", ":"}
)

// NormalizeTitle lowercases, expands "&", drops punctuation and collapses
// whitespace so two spellings of the same title compare equal.
func NormalizeTitle(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "&", " and ")
	value = strings.ReplaceAll(value, "'", "")
	value = strings.ReplaceAll(value, "’", "")
	value = nonAlnumPattern.ReplaceAllString(value, " ")
	return strings.Join(strings.Fields(value), " ")
}

// SignificantTokens returns the words of a title that identify the show, with
// format noise removed. When a title is nothing but noise the full token list
// is returned instead of an empty set.
func SignificantTokens(value string) []string {
	all := strings.Fields(NormalizeTitle(value))
	tokens := make([]string, 0, len(all))
	for _, token := range all {
		if len(token) < 2 {
			continue
		}
		if _, noisy := titleNoise[token]; noisy {
			continue
		}
		if editionTokenPattern.MatchString(token) {
			continue
		}
		tokens = append(tokens, token)
	}
	if len(tokens) == 0 {
		return all
	}
	return tokens
}

// diceCoefficient is the Sørensen–Dice similarity over character bigrams. It
// tolerates word order and small spelling drift, which token sets alone miss.
func diceCoefficient(left, right string) float64 {
	if left == right {
		return 1
	}
	if len(left) < 2 || len(right) < 2 {
		return 0
	}
	bigrams := func(value string) map[string]int {
		counts := make(map[string]int, len(value))
		for i := 0; i+1 < len(value); i++ {
			counts[value[i:i+2]]++
		}
		return counts
	}
	leftBigrams := bigrams(left)
	rightBigrams := bigrams(right)
	overlap := 0
	for pair, count := range leftBigrams {
		if other, ok := rightBigrams[pair]; ok {
			if other < count {
				overlap += other
			} else {
				overlap += count
			}
		}
	}
	total := (len(left) - 1) + (len(right) - 1)
	if total <= 0 {
		return 0
	}
	return 2 * float64(overlap) / float64(total)
}

// TitleSimilarity scores 0..1 for how likely a candidate is the queried show.
// Containment of the query's identifying words carries most of the weight, and
// bigram overlap resolves near misses.
func TitleSimilarity(query, candidate string) float64 {
	queryTokens := SignificantTokens(query)
	candidateTokens := SignificantTokens(candidate)
	if len(queryTokens) == 0 || len(candidateTokens) == 0 {
		return 0
	}
	candidateSet := make(map[string]struct{}, len(candidateTokens))
	for _, token := range candidateTokens {
		candidateSet[token] = struct{}{}
	}
	matched := 0
	for _, token := range queryTokens {
		if _, ok := candidateSet[token]; ok {
			matched++
		} else {
			// Tolerate plural and short-prefix drift: "turncloak"/"turncloaks".
			for existing := range candidateSet {
				if prefixMatch(token, existing) {
					matched++
					break
				}
			}
		}
	}
	containment := float64(matched) / float64(len(queryTokens))
	// Compare the identifying words only; the raw strings differ wildly in
	// length once a disk title carries its whole format description.
	dice := diceCoefficient(strings.Join(queryTokens, " "), strings.Join(candidateTokens, " "))
	return 0.6*containment + 0.4*dice
}

// prefixMatch accepts tokens that differ only by a trailing plural, or where one
// is a prefix of the other and at least four characters long.
func prefixMatch(left, right string) bool {
	if left == right {
		return true
	}
	shorter, longer := left, right
	if len(shorter) > len(longer) {
		shorter, longer = longer, shorter
	}
	if len(shorter) < 4 {
		return false
	}
	if len(longer)-len(shorter) > 2 {
		return false
	}
	return strings.HasPrefix(longer, shorter) || strings.HasPrefix(shorter, longer)
}

// QueryVariants builds the searches to try, loosest last. Apple's search is
// ranked, so a long disk title returns nothing while its first clause returns
// the show.
func QueryVariants(query string) []string {
	var variants []string
	add := func(value string) {
		value = strings.Join(strings.Fields(value), " ")
		if value == "" {
			return
		}
		for _, existing := range variants {
			if strings.EqualFold(existing, value) {
				return
			}
		}
		variants = append(variants, value)
	}

	add(query)
	for _, separator := range variantSplitter {
		if index := strings.Index(query, separator); index > 2 {
			add(query[:index])
			break
		}
	}
	tokens := SignificantTokens(query)
	if len(tokens) > 3 {
		add(strings.Join(tokens[:3], " "))
	}
	if len(tokens) > 0 {
		longest := tokens[0]
		for _, token := range tokens[1:] {
			if len(token) > len(longest) {
				longest = token
			}
		}
		add(longest)
	}
	return variants
}

// ScoredShow is a directory entry with the confidence that it answers a query.
type ScoredShow struct {
	Show
	Score float64
}

// rankCandidates dedupes by collection id, scores against the original query and
// returns the strong matches, best first.
func rankCandidates(query string, shows []Show, floor float64, limit int) []ScoredShow {
	best := make(map[string]ScoredShow, len(shows))
	for _, show := range shows {
		if show.Title == "" {
			continue
		}
		// Score the show name and the name with its publisher, since a title on
		// disk sometimes carries the network instead of the show.
		score := TitleSimilarity(query, show.Title)
		if combined := TitleSimilarity(query, show.Title+" "+show.Artist); combined > score {
			score = combined
		}
		existing, seen := best[show.AppleID]
		if seen && existing.Score >= score {
			continue
		}
		best[show.AppleID] = ScoredShow{Show: show, Score: score}
	}

	scored := make([]ScoredShow, 0, len(best))
	for _, candidate := range best {
		if candidate.Score < floor {
			continue
		}
		scored = append(scored, candidate)
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].Score == scored[j].Score {
			return scored[i].Title < scored[j].Title
		}
		return scored[i].Score > scored[j].Score
	})
	if limit > 0 && len(scored) > limit {
		scored = scored[:limit]
	}
	return scored
}
