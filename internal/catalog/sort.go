package catalog

import (
	"cmp"
	"slices"
	"strconv"
)

// Sort orders items by year, title, date added or size, in dir (Asc or
// Desc; "" is the order's natural direction). Items without a year go last
// either way. Ties are broken by title and then by id, so the order is
// stable between requests.
func Sort(items []Item, order, dir string) {
	if dir == "" {
		dir = Desc
		if order == OrderTitle {
			dir = Asc
		}
	}
	slices.SortStableFunc(items, func(a, b Item) int {
		if order == OrderYear && (a.Year == 0) != (b.Year == 0) {
			if a.Year == 0 {
				return 1
			}
			return -1
		}
		var c int
		switch order {
		case OrderTitle:
			c = cmp.Compare(a.norm, b.norm)
		case OrderAdded:
			c = cmp.Compare(a.Added, b.Added)
		case OrderSize:
			c = cmp.Compare(a.Size, b.Size)
		case OrderListAdded:
			c = cmp.Compare(a.listAdded, b.listAdded)
		default:
			c = cmp.Compare(a.Year, b.Year)
		}
		if dir == Desc {
			c = -c
		}
		if c != 0 {
			return c
		}
		if c = cmp.Compare(a.norm, b.norm); c != 0 {
			return c
		}
		return cmp.Compare(a.id(), b.id())
	})
}

// id tells items apart: a movie's TMDB id or a version's key.
func (it *Item) id() string {
	if it.Kind == KindMovie {
		return "movie:" + strconv.Itoa(it.TMDBID)
	}
	return it.Key
}

// SortQuery sorts items as q asks, including by date added to q's list.
func SortQuery(items []Item, q Query) {
	if q.Order == OrderListAdded {
		for i := range items {
			items[i].listAdded, _ = items[i].addedTo(q.Facets[FacetList])
		}
	}
	Sort(items, q.Order, q.Dir)
}
