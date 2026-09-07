package sql

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/leganck/go-devops/devops"
)

type ExportRequest struct {
	EnvName        string
	DatasourceName string
	SQL            string
	Format         string // csv | jsonl
	Limit          int
}

func (c *Client) Export(ctx context.Context, req ExportRequest, w io.Writer) error {
	if err := CheckReadOnly(req.SQL); err != nil {
		return err
	}
	if w == nil {
		return devops.NewError(devops.KindInvalidArgument, "writer is required", nil)
	}
	format := strings.ToLower(strings.TrimSpace(req.Format))
	if format == "" {
		format = "csv"
	}
	if format != "csv" && format != "jsonl" {
		return devops.NewError(devops.KindInvalidArgument, "format must be csv or jsonl", nil)
	}

	pageSize := c.normalizePage(req.Limit)
	page := 1
	var csvW *csv.Writer
	headerWritten := false
	var columns []string
	maxPages := c.maxPages
	if maxPages <= 0 {
		maxPages = 50
	}
	for page <= maxPages {
		res, err := c.execRaw(ctx, ExecRequest{
			EnvName:        req.EnvName,
			DatasourceName: req.DatasourceName,
			SQL:            req.SQL,
			Page:           page,
			Limit:          pageSize,
			UniqueID:       newUniqueID(),
		})
		if err != nil {
			return err
		}
		if !res.IsSelect() {
			return devops.NewError(devops.KindInvalidArgument, "export requires a SELECT", nil)
		}
		if !headerWritten {
			columns = res.Columns
			if len(columns) == 0 && len(res.Rows) > 0 {
				for k := range res.Rows[0] {
					columns = append(columns, k)
				}
			}
			if format == "csv" {
				csvW = csv.NewWriter(w)
				if err := csvW.Write(columns); err != nil {
					return err
				}
			}
			headerWritten = true
		}
		for _, row := range res.Rows {
			if format == "jsonl" {
				if err := json.NewEncoder(w).Encode(row); err != nil {
					return err
				}
				continue
			}
			rec := make([]string, len(columns))
			for i, col := range columns {
				rec[i] = fmt.Sprint(nullString(row[col]))
			}
			if err := csvW.Write(rec); err != nil {
				return err
			}
		}
		if csvW != nil {
			csvW.Flush()
			if err := csvW.Error(); err != nil {
				return err
			}
		}
		if !res.HasMore || len(res.Rows) == 0 {
			break
		}
		page++
	}
	return nil
}

func nullString(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}
