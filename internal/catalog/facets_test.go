package catalog

import (
	"net/url"
	"reflect"
	"sort"
	"testing"
)

func ids(items []Item) []string {
	out := []string{}
	for _, it := range items {
		out = append(out, it.id())
	}
	sort.Strings(out)
	return out
}

func TestParseQuery(t *testing.T) {
	cases := []struct {
		query string
		want  Query
	}{
		{"", Query{Facets: map[string]string{}, Order: OrderYear, Dir: Desc}},
		{"decada=1970&pais=IT&orden=titulo", Query{Facets: map[string]string{"decada": "1970", "pais": "IT"}, Order: OrderTitle, Dir: Asc}},
		{"orden=tamano&dir=asc&estado=copia-identica", Query{Facets: map[string]string{"estado": "copia-identica"}, Order: OrderSize, Dir: Asc}},
		// Malformed values and unknown parameters are dropped.
		{"decada=1975&anio=x&director=0&pais=it&idioma=Italian&resolucion=8K&estado=roto&genero=%20&foo=1&orden=nada&dir=up",
			Query{Facets: map[string]string{}, Order: OrderYear, Dir: Desc}},
		{"director=4415&coleccion=99&idioma=it&subs=spa&ubicacion=cine/1970s&resolucion=4K&genero=Drama&anio=1973",
			Query{Facets: map[string]string{"director": "4415", "coleccion": "99", "idioma": "it", "subs": "spa",
				"ubicacion": "cine/1970s", "resolucion": "4K", "genero": "Drama", "anio": "1973"}, Order: OrderYear, Dir: Desc}},
	}
	for _, c := range cases {
		v, _ := url.ParseQuery(c.query)
		if got := ParseQuery(v); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%q: got %+v, want %+v", c.query, got, c.want)
		}
	}
}

func TestFilter(t *testing.T) {
	items := Items(snapshot(), roots)
	cases := []struct {
		facets map[string]string
		want   []string
	}{
		{map[string]string{}, []string{"id:9", "movie:7857", "movie:7858", "n1", "p1", "s1"}},
		{map[string]string{"decada": "1970"}, []string{"movie:7857", "movie:7858", "n1", "p1", "s1"}},
		{map[string]string{"anio": "1972"}, []string{"movie:7858", "p1"}},
		// TMDB facets only see identified movies.
		{map[string]string{"director": "4415"}, []string{"movie:7857", "movie:7858"}},
		{map[string]string{"genero": "Comedia"}, []string{"movie:7857"}},
		{map[string]string{"pais": "FR"}, []string{"movie:7857"}},
		{map[string]string{"idioma": "it"}, []string{"movie:7857", "movie:7858"}},
		{map[string]string{"coleccion": "99"}, []string{"movie:7858"}},
		// Any version counts: Amarcord has an SD version too.
		{map[string]string{"resolucion": "SD"}, []string{"movie:7857", "s1"}},
		{map[string]string{"subs": "es"}, []string{"movie:7857"}},
		{map[string]string{"ubicacion": "cine-ordenar"}, []string{"id:9", "movie:7857", "p1", "s1"}},
		{map[string]string{"ubicacion": "cine/1970s"}, []string{"movie:7857", "movie:7858", "n1"}},
		{map[string]string{"estado": "sin-identificar"}, []string{"id:9", "n1", "p1", "s1"}},
		{map[string]string{"estado": "varias-versiones"}, []string{"movie:7857"}},
		{map[string]string{"estado": "copia-identica"}, []string{"movie:7857"}},
		{map[string]string{"decada": "1970", "director": "4415", "resolucion": "1080p"}, []string{"movie:7857", "movie:7858"}},
		{map[string]string{"director": "1"}, []string{}},
	}
	for _, c := range cases {
		if got := ids(Filter(items, c.facets)); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%v: got %v, want %v", c.facets, got, c.want)
		}
	}
}

func TestCounts(t *testing.T) {
	items := Items(snapshot(), roots)
	counts := Counts(items, map[string]string{"pais": "IT", "resolucion": "1080p"})
	// Each menu counts the items that match the other facets.
	if got, want := counts["resolucion"], []FacetValue{{"1080p", "1080p", 2}, {"SD", "SD", 1}}; !reflect.DeepEqual(got, want) {
		t.Errorf("resolucion %v, want %v", got, want)
	}
	if got, want := counts["pais"], []FacetValue{{"IT", "IT", 2}, {"FR", "FR", 1}}; !reflect.DeepEqual(got, want) {
		t.Errorf("pais %v, want %v", got, want)
	}
	if got, want := counts["director"], []FacetValue{{"4415", "Federico Fellini", 2}}; !reflect.DeepEqual(got, want) {
		t.Errorf("director %v, want %v", got, want)
	}
	if got, want := counts["coleccion"], []FacetValue{{"99", "Fellini", 1}}; !reflect.DeepEqual(got, want) {
		t.Errorf("coleccion %v, want %v", got, want)
	}
	if got, want := counts["genero"], []FacetValue{{"Drama", "Drama", 2}, {"Comedia", "Comedia", 1}}; !reflect.DeepEqual(got, want) {
		t.Errorf("genero %v, want %v", got, want)
	}

	all := Counts(items, map[string]string{})
	if got, want := all["decada"], []FacetValue{{"1970", "1970s", 5}}; !reflect.DeepEqual(got, want) {
		t.Errorf("decada %v, want %v", got, want)
	}
	if got, want := all["anio"], []FacetValue{{"1979", "1979", 1}, {"1976", "1976", 1}, {"1973", "1973", 1}, {"1972", "1972", 2}}; !reflect.DeepEqual(got, want) {
		t.Errorf("anio %v, want %v", got, want)
	}
	if got, want := all["ubicacion"], []FacetValue{{"cine", "cine", 3}, {"cine-ordenar", "cine-ordenar", 4},
		{"cine-ordenar/Tarkovsky", "cine-ordenar/Tarkovsky", 1}, {"cine/1970s", "cine/1970s", 3}, {"cine/Collections", "cine/Collections", 1}}; !reflect.DeepEqual(got, want) {
		t.Errorf("ubicacion %v, want %v", got, want)
	}
	if got, want := all["estado"], []FacetValue{{"sin-identificar", "sin-identificar", 4}, {"copia-identica", "copia-identica", 1},
		{"varias-versiones", "varias-versiones", 1}}; !reflect.DeepEqual(got, want) {
		t.Errorf("estado %v, want %v", got, want)
	}
	if len(all) != len(FacetNames) {
		t.Errorf("facets %d, want %d", len(all), len(FacetNames))
	}
	for _, f := range FacetNames {
		if all[f] == nil {
			t.Errorf("%s: nil instead of an empty list", f)
		}
	}
}

func TestSort(t *testing.T) {
	items := Items(snapshot(), roots)
	order := func(o, dir string) []string {
		Sort(items, o, dir)
		out := []string{}
		for _, it := range items {
			out = append(out, it.id())
		}
		return out
	}
	cases := []struct {
		order, dir string
		want       []string
	}{
		// Without a year last; Roma and Solaris tie on 1972 and go by title.
		{OrderYear, "", []string{"s1", "n1", "movie:7857", "movie:7858", "p1", "id:9"}},
		{OrderYear, Asc, []string{"movie:7858", "p1", "movie:7857", "n1", "s1", "id:9"}},
		{OrderTitle, "", []string{"movie:7857", "n1", "movie:7858", "p1", "s1", "id:9"}},
		{OrderTitle, Desc, []string{"id:9", "s1", "p1", "movie:7858", "n1", "movie:7857"}},
		{OrderAdded, "", []string{"p1", "id:9", "s1", "n1", "movie:7857", "movie:7858"}},
		{OrderSize, "", []string{"movie:7857", "movie:7858", "p1", "s1", "n1", "id:9"}},
	}
	for _, c := range cases {
		if got := order(c.order, c.dir); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s %s: got %v, want %v", c.order, c.dir, got, c.want)
		}
	}
}
