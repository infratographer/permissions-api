package api

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
)

var (
	// MaxPaginationSize represents the maximum number of records that can be returned per page.
	MaxPaginationSize = 1000
	// DefaultPaginationSize represents the default number of records that are returned per page.
	DefaultPaginationSize = 100
)

// Pagination allow you to paginate the results.
type Pagination struct {
	Limit int
	Page  int
	Order string
}

func ParsePagination(c *gin.Context) (*Pagination, error) {
	// Initializing default
	limit := DefaultPaginationSize
	page := 1
	order := ""
	query := c.Request.URL.Query()

	for key, value := range query {
		queryValue := value[len(value)-1]

		var err error
		switch key {
		case "limit":
			limit, err = strconv.Atoi(queryValue)
		case "page":
			page, err = strconv.Atoi(queryValue)
		case "order":
			order = queryValue
		}

		if err != nil {
			return nil, fmt.Errorf("invalid pagination query: %w", err)
		}
	}

	return &Pagination{
		Limit: parseLimit(limit),
		Page:  page,
		Order: order,
	}, nil
}

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

func (p *Pagination) SetHeaders(c *gin.Context, count int) {
	c.Header("Pagination-Count", strconv.Itoa(count))
	c.Header("Pagination-Limit", strconv.Itoa(p.Limit))
	c.Header("Pagination-Page", strconv.Itoa(p.Page))
}
