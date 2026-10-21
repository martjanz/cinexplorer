package identify

import (
	"reflect"
	"testing"
)

func TestReadings(t *testing.T) {
	cases := []struct {
		in   Query
		want []Query
	}{
		{Query{Title: "Azor - Andreas Fontana", Year: 2021, Runtime: 100}, []Query{
			{Title: "Azor", Director: "Andreas Fontana", Year: 2021, Runtime: 100},
			{Title: "Andreas Fontana", Director: "Azor", Year: 2021, Runtime: 100},
		}},
		// En dash; the comma's right side has 6 words, so no comma reading.
		{Query{Title: "Devoto, la invasión silenciosa – Martín Basterretche"}, []Query{
			{Title: "Devoto, la invasión silenciosa", Director: "Martín Basterretche"},
			{Title: "Martín Basterretche", Director: "Devoto, la invasión silenciosa"},
		}},
		{Query{Title: "C.Chabrol-Madame Bovary", Year: 1991}, []Query{
			{Title: "C.Chabrol", Director: "Madame Bovary", Year: 1991},
			{Title: "Madame Bovary", Director: "C.Chabrol", Year: 1991},
		}},
		{Query{Title: "Melancholia, Lars Von Trier", Year: 2011}, []Query{
			{Title: "Melancholia", Director: "Lars Von Trier", Year: 2011},
		}},
		{Query{Title: "L'ivresse du pouvoir,C Chabrol", Year: 2005}, []Query{
			{Title: "L'ivresse du pouvoir", Director: "C Chabrol", Year: 2005},
		}},
		{Query{Title: "300 - Rise of an Empire", Year: 2014}, nil}, // digit side: no dash reading, and a dash blocks trailing words
		{Query{Title: "Mediterraneo G Salvatore", Year: 1991}, []Query{
			{Title: "Mediterraneo G", Director: "Salvatore", Year: 1991},
			{Title: "Mediterraneo", Director: "G Salvatore", Year: 1991},
		}},
		{Query{Title: "Loulou Pialat", Year: 1980}, []Query{{Title: "Loulou", Director: "Pialat", Year: 1980}}},
		{Query{Title: "Sebastián Díaz", Director: "muerte y profanación 4 lonkos： Vida", Year: 2020}, []Query{
			{Title: "muerte y profanación 4 lonkos： Vida", Director: "Sebastián Díaz", Year: 2020},
		}},
		{Query{Title: "Spider-Man"}, nil},
		{Query{Title: "X-Men"}, nil},
		{Query{Title: "Jean-Luc"}, nil},
		{Query{Title: "Amarcord"}, nil},
		{Query{Title: "La notte"}, nil}, // 2 words, second lowercase
	}
	for _, c := range cases {
		got := readings(c.in)
		if len(got) == 0 && len(c.want) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("readings(%+v) = %+v, want %+v", c.in, got, c.want)
		}
	}
}
