package api

import (
	"context"

	"nstock/internal/ncode"

	"github.com/gin-gonic/gin"
)

type queryModel struct {
	Kind       string
	Permission string
	Filters    []string
	Sorts      []string
	Exec       func(context.Context, ncode.QueryRequest) (any, int64, error)
}

func (s *Server) queryModels() map[string]queryModel {
	return map[string]queryModel{
		"pageForOrder": {
			Kind: "page", Permission: "stock:order:view",
			Filters: []string{"code"}, Sorts: []string{"createdAt"},
			Exec: func(ctx context.Context, req ncode.QueryRequest) (any, int64, error) {
				rows, _, _, total, err := s.orderPage(ncode.AsString(req.Filters["code"]), req.Page, req.Size, ncode.Ascending(req.Sort))
				return rows, total, err
			},
		},
		"pageForPrediction": {
			Kind: "page", Permission: "stock:predict:view",
			Filters: []string{"code"}, Sorts: []string{"createdAt"},
			Exec: func(ctx context.Context, req ncode.QueryRequest) (any, int64, error) {
				rows, _, _, total, err := s.predictionPage(ncode.AsString(req.Filters["code"]), req.Page, req.Size, ncode.Ascending(req.Sort))
				return rows, total, err
			},
		},
		"pageForAlertEvent": {
			Kind: "page", Permission: "stock:alert:view",
			Filters: []string{"code"}, Sorts: []string{"createdAt"},
			Exec: func(ctx context.Context, req ncode.QueryRequest) (any, int64, error) {
				rows, _, _, total, err := s.eventPage(ncode.AsString(req.Filters["code"]), req.Page, req.Size, ncode.Ascending(req.Sort))
				return rows, total, err
			},
		},
		"pageForBacktest": {
			Kind: "page", Permission: "stock:strategy:view",
			Filters: []string{"strategyId"}, Sorts: []string{"createdAt"},
			Exec: func(ctx context.Context, req ncode.QueryRequest) (any, int64, error) {
				rows, _, _, total, err := s.backtestPage(ncode.AsUint(req.Filters["strategyId"]), req.Page, req.Size, ncode.Ascending(req.Sort))
				return rows, total, err
			},
		},
		"listWatch": {
			Kind: "list", Permission: "stock:watch:view",
			Exec: func(context.Context, ncode.QueryRequest) (any, int64, error) {
				rows, err := s.watchData()
				return rows, 0, err
			},
		},
		"listStrategy": {
			Kind: "list", Permission: "stock:strategy:view",
			Exec: func(context.Context, ncode.QueryRequest) (any, int64, error) {
				rows, err := s.strategyData()
				return rows, 0, err
			},
		},
		"listAlert": {
			Kind: "list", Permission: "stock:alert:view",
			Exec: func(context.Context, ncode.QueryRequest) (any, int64, error) {
				rows, err := s.alertData()
				return rows, 0, err
			},
		},
		"listIndex": {
			Kind: "list", Permission: "stock:quote:view",
			Exec: func(ctx context.Context, _ ncode.QueryRequest) (any, int64, error) {
				rows, err := s.indicesData(ctx)
				return rows, 0, err
			},
		},
		"listQuote": {
			Kind: "list", Permission: "stock:quote:view",
			Filters: []string{"refresh"},
			Exec: func(ctx context.Context, req ncode.QueryRequest) (any, int64, error) {
				data, err := s.quotesData(ctx, ncode.AsBool(req.Filters["refresh"]))
				return data, 0, err
			},
		},
		"oneAccount": {
			Kind: "one", Permission: "stock:account:view",
			Exec: func(ctx context.Context, _ ncode.QueryRequest) (any, int64, error) {
				data, err := s.accountData(ctx)
				return data, 0, err
			},
		},
		"oneQuote": {
			Kind: "one", Permission: "stock:quote:view",
			Filters: []string{"code", "refresh"},
			Exec: func(ctx context.Context, req ncode.QueryRequest) (any, int64, error) {
				data, err := s.quoteData(ctx, ncode.AsString(req.Filters["code"]), ncode.AsBool(req.Filters["refresh"]))
				return data, 0, err
			},
		},
		"oneKline": {
			Kind: "one", Permission: "stock:quote:view",
			Filters: []string{"code", "size", "refresh"},
			Exec: func(ctx context.Context, req ncode.QueryRequest) (any, int64, error) {
				refresh := true
				if _, ok := req.Filters["refresh"]; ok {
					refresh = ncode.AsBool(req.Filters["refresh"])
				}
				data, err := s.klineData(ctx, ncode.AsString(req.Filters["code"]), ncode.AsInt(req.Filters["size"]), refresh)
				return data, 0, err
			},
		},
		"oneMonitor": {
			Kind: "one", Permission: "stock:monitor:view",
			Exec: func(context.Context, ncode.QueryRequest) (any, int64, error) {
				return s.monitorData(), 0, nil
			},
		},
	}
}

func (s *Server) querySearch(c *gin.Context) { s.runQuery(c, "page") }
func (s *Server) queryList(c *gin.Context)   { s.runQuery(c, "list") }
func (s *Server) queryOne(c *gin.Context)    { s.runQuery(c, "one") }

func (s *Server) runQuery(c *gin.Context, kind string) {
	raw, err := readBody(c)
	if err != nil {
		ncode.Write(c.Writer, err, nil)
		return
	}
	if err = ncode.CheckTenant(raw); err != nil {
		ncode.Write(c.Writer, err, nil)
		return
	}
	var req ncode.QueryRequest
	if err = bind(raw, &req); err != nil {
		ncode.Write(c.Writer, err, nil)
		return
	}
	model, ok := s.models[req.Model]
	if !ok || req.Model == "" {
		ncode.Write(c.Writer, ncode.Fail(ncode.CodeNotFound, "查询模型不存在"), nil)
		return
	}
	if model.Kind != kind {
		ncode.Write(c.Writer, ncode.Fail(ncode.CodeBadRequest, "查询模型与接口不匹配"), nil)
		return
	}
	if err = s.authorize(c, model.Permission); err != nil {
		ncode.Write(c.Writer, err, nil)
		return
	}
	filters, err := ncode.SanitizeFilters(req.Filters, model.Filters)
	if err != nil {
		ncode.Write(c.Writer, err, nil)
		return
	}
	if err = ncode.CheckSort(req.Sort, model.Sorts); err != nil {
		ncode.Write(c.Writer, err, nil)
		return
	}
	req.Filters = filters
	req.Page, req.Size = ncode.NormalizePage(req.Page, req.Size)
	data, total, err := model.Exec(c.Request.Context(), req)
	if kind == "page" {
		ncode.WritePage(c.Writer, err, data, req.Page, req.Size, total)
		return
	}
	ncode.Write(c.Writer, err, data)
}
