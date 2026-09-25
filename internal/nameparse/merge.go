package nameparse

// Merge returns the more informative of two parses (e.g. file name vs folder
// name), filling its empty fields from the other. On a tie a wins.
func Merge(a, b Parsed) Parsed {
	if score(b) > score(a) {
		a, b = b, a
	}
	if a.Year == 0 {
		a.Year = b.Year
	}
	if a.Director == "" {
		a.Director = b.Director
	}
	if len(a.Countries) == 0 {
		a.Countries = b.Countries
	}
	fill := func(dst *string, src string) {
		if *dst == "" {
			*dst = src
		}
	}
	fill(&a.Resolution, b.Resolution)
	fill(&a.Source, b.Source)
	fill(&a.Codec, b.Codec)
	fill(&a.Language, b.Language)
	fill(&a.Group, b.Group)
	fill(&a.IMDbID, b.IMDbID)
	return a
}

func score(p Parsed) int {
	s := 0
	if p.Year != 0 {
		s += 2
	}
	if p.Director != "" {
		s++
	}
	return s
}
