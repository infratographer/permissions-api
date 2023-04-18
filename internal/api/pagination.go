package api

import (
	"strconv"

	"github.com/labstack/echo/v4"
)

var (
	// MaxPaginationSize represents the maximum number of records that can be returned per page
	MaxPaginationSize = 1000
	// DefaultPaginationSize represents the default number of records that are returned per page
	DefaultPaginationSize = 100
)

// Pagination allow you to paginate the results
type Pagination struct {
	Limit int
	Page  int
	Order string
}

// ParsePagination parses the pagination query parameters from the echo context
func ParsePagination(c echo.Context) *Pagination {
	// Initializing default
	limit := DefaultPaginationSize
	page := 1
	order := ""
	query := c.Request().URL.Query()

	for key, value := range query {
		queryValue := value[len(value)-1]

		switch key {
		case "limit":
			limit, _ = strconv.Atoi(queryValue)
		case "page":
			page, _ = strconv.Atoi(queryValue)
		case "order":
			order = queryValue
		}
	}

	return &Pagination{
		Limit: parseLimit(limit),
		Page:  page,
		Order: order,
	}
}

// // queryMods converts the list params into sql conditions that can be added to sql queries
// func (p *Pagination) QueryMods() []qm.QueryMod {
// 	if p == nil {
// 		p = &Pagination{}
// 	}

// 	mods := []qm.QueryMod{}

// 	mods = append(mods, qm.Limit(p.Limit))

// 	if p.Page != 0 {
// 		mods = append(mods, qm.Offset(p.offset()))
// 	}

// 	return mods
// }

func parseLimit(l int) int {
	limit := l

	switch {
	case limit > MaxPaginationSize:
		limit = MaxPaginationSize
	case limit <= 0:
		limit = DefaultPaginationSize
	}

	return limit
}

// func (p *Pagination) offset() int {
// 	page := p.Page
// 	if page == 0 {
// 		page = 1
// 	}
//
// 	return (page - 1) * p.Limit
// }

// SetHeaders sets the pagination headers on a response
func (p *Pagination) SetHeaders(c echo.Context, count int) {
	c.Response().Header().Set("Pagination-Count", strconv.Itoa(count))
	c.Response().Header().Set("Pagination-Limit", strconv.Itoa(p.Limit))
	c.Response().Header().Set("Pagination-Page", strconv.Itoa(p.Page))
}
