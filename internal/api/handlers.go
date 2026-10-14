package api

import (
	"context"
	"math"

	"nstock/internal/ncode"
	"nstock/internal/quant"
	"nstock/internal/service"

	"github.com/gin-gonic/gin"
)

type emptyRequest struct{}

type keywordRequest struct {
	Keyword string `json:"keyword"`
}

type quoteListRequest struct {
	Refresh bool `json:"refresh"`
}

type quoteGetRequest struct {
	Code    string `json:"code"`
	Refresh bool   `json:"refresh"`
}

type klineGetRequest struct {
	Code    string `json:"code"`
	Size    int    `json:"size"`
	Refresh *bool  `json:"refresh"`
}

type codeRequest struct {
	Code string `json:"code"`
}

type pageRequest struct {
	Code       string      `json:"code"`
	StrategyID uint        `json:"strategyId"`
	Page       int         `json:"page"`
	Size       int         `json:"size"`
	Sort       *ncode.Sort `json:"sort"`
}

type orderCreateRequest struct {
	Code string  `json:"code"`
	Side string  `json:"side"`
	Qty  float64 `json:"qty"`
}

type strategyCreateRequest struct {
	Name    string       `json:"name"`
	Type    string       `json:"type"`
	Code    string       `json:"code"`
	Enabled *bool        `json:"enabled"`
	Params  quant.Params `json:"params"`
}

type strategyUpdateRequest struct {
	ID      uint          `json:"id"`
	Name    *string       `json:"name"`
	Code    *string       `json:"code"`
	Enabled *bool         `json:"enabled"`
	Params  *quant.Params `json:"params"`
}

type idRequest struct {
	ID uint `json:"id"`
}

type alertCreateRequest struct {
	Code      string  `json:"code"`
	Kind      string  `json:"kind"`
	Threshold float64 `json:"threshold"`
	Name      string  `json:"name"`
}

type alertUpdateRequest struct {
	ID        uint     `json:"id"`
	Name      *string  `json:"name"`
	Enabled   *bool    `json:"enabled"`
	Threshold *float64 `json:"threshold"`
}

func (s *Server) health(c *gin.Context) {
	s.handle(c, "stock:system:view", &emptyRequest{}, func(context.Context) (any, error) {
		return gin.H{"service": "nstock", "dataSource": "eastmoney"}, nil
	})
}

func (s *Server) search(c *gin.Context) {
	var req keywordRequest
	s.handle(c, "stock:quote:view", &req, func(ctx context.Context) (any, error) {
		if req.Keyword == "" {
			return nil, ncode.Fail(ncode.CodeBadRequest, "请输入代码或名称")
		}
		hits, err := s.svc.Search(ctx, req.Keyword)
		if err != nil {
			return nil, err
		}
		return toHits(hits), nil
	})
}

func (s *Server) indices(c *gin.Context) {
	s.handle(c, "stock:quote:view", &emptyRequest{}, func(ctx context.Context) (any, error) {
		return s.indicesData(ctx)
	})
}

func (s *Server) quoteList(c *gin.Context) {
	var req quoteListRequest
	s.handle(c, "stock:quote:view", &req, func(ctx context.Context) (any, error) {
		return s.quotesData(ctx, req.Refresh)
	})
}

func (s *Server) quoteGet(c *gin.Context) {
	var req quoteGetRequest
	s.handle(c, "stock:quote:view", &req, func(ctx context.Context) (any, error) {
		return s.quoteData(ctx, req.Code, req.Refresh)
	})
}

func (s *Server) klineGet(c *gin.Context) {
	var req klineGetRequest
	s.handle(c, "stock:quote:view", &req, func(ctx context.Context) (any, error) {
		refresh := true
		if req.Refresh != nil {
			refresh = *req.Refresh
		}
		return s.klineData(ctx, req.Code, req.Size, refresh)
	})
}

func (s *Server) watchList(c *gin.Context) {
	s.handle(c, "stock:watch:view", &emptyRequest{}, func(context.Context) (any, error) {
		return s.watchData()
	})
}

func (s *Server) watchCreate(c *gin.Context) {
	var req codeRequest
	s.handle(c, "stock:watch:create", &req, func(ctx context.Context) (any, error) {
		if req.Code == "" {
			return nil, ncode.Fail(ncode.CodeBadRequest, "请提供证券代码或名称")
		}
		item, err := s.svc.AddWatch(ctx, req.Code)
		if err != nil {
			return nil, err
		}
		return toWatch(item), nil
	})
}

func (s *Server) watchDelete(c *gin.Context) {
	var req codeRequest
	s.handle(c, "stock:watch:delete", &req, func(context.Context) (any, error) {
		if err := s.svc.RemoveWatch(req.Code); err != nil {
			return nil, err
		}
		return nil, nil
	})
}

func (s *Server) predictExecute(c *gin.Context) {
	var req codeRequest
	s.handle(c, "stock:predict:execute", &req, func(ctx context.Context) (any, error) {
		view, err := s.svc.Predict(ctx, req.Code)
		if err != nil {
			return nil, err
		}
		return toPrediction(view), nil
	})
}

func (s *Server) predictSearch(c *gin.Context) {
	var req pageRequest
	s.handlePage(c, "stock:predict:view", &req, func(context.Context) (any, int, int, int64, error) {
		return s.predictionPage(req.Code, req.Page, req.Size, ncode.Ascending(req.Sort))
	})
}

func (s *Server) accountGet(c *gin.Context) {
	s.handle(c, "stock:account:view", &emptyRequest{}, func(ctx context.Context) (any, error) {
		return s.accountData(ctx)
	})
}

func (s *Server) accountReset(c *gin.Context) {
	s.handle(c, "stock:account:reset", &emptyRequest{}, func(ctx context.Context) (any, error) {
		view, err := s.svc.ResetAccount(ctx)
		if err != nil {
			return nil, err
		}
		return toAccount(view), nil
	})
}

func (s *Server) orderCreate(c *gin.Context) {
	var req orderCreateRequest
	s.handle(c, "stock:order:create", &req, func(ctx context.Context) (any, error) {
		order, err := s.svc.PlaceOrder(ctx, service.OrderInput{
			Code: req.Code, Side: req.Side, Qty: int(math.Round(req.Qty)), Source: "manual",
		})
		if err != nil {
			return nil, err
		}
		return toOrder(order), nil
	})
}

func (s *Server) orderSearch(c *gin.Context) {
	var req pageRequest
	s.handlePage(c, "stock:order:view", &req, func(context.Context) (any, int, int, int64, error) {
		return s.orderPage(req.Code, req.Page, req.Size, ncode.Ascending(req.Sort))
	})
}

func (s *Server) strategyList(c *gin.Context) {
	s.handle(c, "stock:strategy:view", &emptyRequest{}, func(context.Context) (any, error) {
		return s.strategyData()
	})
}

func (s *Server) strategyCreate(c *gin.Context) {
	var req strategyCreateRequest
	s.handle(c, "stock:strategy:create", &req, func(ctx context.Context) (any, error) {
		if req.Code == "" || req.Type == "" {
			return nil, ncode.Fail(ncode.CodeBadRequest, "请填写策略类型和证券")
		}
		enabled := true
		if req.Enabled != nil {
			enabled = *req.Enabled
		}
		view, err := s.svc.CreateStrategy(ctx, service.StrategyInput{
			Name: req.Name, Type: req.Type, Code: req.Code, Params: req.Params, Enabled: enabled,
		})
		if err != nil {
			return nil, err
		}
		return toStrategy(view), nil
	})
}

func (s *Server) strategyUpdate(c *gin.Context) {
	var req strategyUpdateRequest
	s.handle(c, "stock:strategy:update", &req, func(ctx context.Context) (any, error) {
		if req.ID == 0 {
			return nil, ncode.Fail(ncode.CodeBadRequest, "请提供策略编号")
		}
		view, err := s.svc.UpdateStrategy(ctx, req.ID, service.StrategyPatch{
			Name: req.Name, Code: req.Code, Enabled: req.Enabled, Params: req.Params,
		})
		if err != nil {
			return nil, err
		}
		return toStrategy(view), nil
	})
}

func (s *Server) strategyDelete(c *gin.Context) {
	var req idRequest
	s.handle(c, "stock:strategy:delete", &req, func(context.Context) (any, error) {
		if req.ID == 0 {
			return nil, ncode.Fail(ncode.CodeBadRequest, "请提供策略编号")
		}
		return nil, s.svc.DeleteStrategy(req.ID)
	})
}

func (s *Server) strategyBacktest(c *gin.Context) {
	var req idRequest
	s.handle(c, "stock:strategy:backtest", &req, func(ctx context.Context) (any, error) {
		if req.ID == 0 {
			return nil, ncode.Fail(ncode.CodeBadRequest, "请提供策略编号")
		}
		view, err := s.svc.Backtest(ctx, req.ID)
		if err != nil {
			return nil, err
		}
		return toBacktest(view), nil
	})
}

func (s *Server) strategyExecute(c *gin.Context) {
	var req idRequest
	s.handle(c, "stock:strategy:execute", &req, func(ctx context.Context) (any, error) {
		if req.ID == 0 {
			return nil, ncode.Fail(ncode.CodeBadRequest, "请提供策略编号")
		}
		view, err := s.svc.RunStrategy(ctx, req.ID)
		if err != nil {
			return nil, err
		}
		return toRun(view), nil
	})
}

func (s *Server) strategyExecuteEnabled(c *gin.Context) {
	s.handle(c, "stock:strategy:execute", &emptyRequest{}, func(ctx context.Context) (any, error) {
		rows, err := s.svc.RunEnabled(ctx)
		if err != nil {
			return nil, err
		}
		return toRuns(rows), nil
	})
}

func (s *Server) backtestSearch(c *gin.Context) {
	var req pageRequest
	s.handlePage(c, "stock:strategy:view", &req, func(context.Context) (any, int, int, int64, error) {
		return s.backtestPage(req.StrategyID, req.Page, req.Size, ncode.Ascending(req.Sort))
	})
}

func (s *Server) alertList(c *gin.Context) {
	s.handle(c, "stock:alert:view", &emptyRequest{}, func(context.Context) (any, error) {
		return s.alertData()
	})
}

func (s *Server) alertCreate(c *gin.Context) {
	var req alertCreateRequest
	s.handle(c, "stock:alert:create", &req, func(ctx context.Context) (any, error) {
		row, err := s.svc.CreateRule(ctx, service.RuleInput{
			Code: req.Code, Kind: req.Kind, Threshold: req.Threshold, Name: req.Name,
		})
		if err != nil {
			return nil, err
		}
		return toAlert(row), nil
	})
}

func (s *Server) alertUpdate(c *gin.Context) {
	var req alertUpdateRequest
	s.handle(c, "stock:alert:update", &req, func(context.Context) (any, error) {
		if req.ID == 0 {
			return nil, ncode.Fail(ncode.CodeBadRequest, "请提供盯盘编号")
		}
		row, err := s.svc.UpdateRule(req.ID, service.RulePatch{
			Name: req.Name, Enabled: req.Enabled, Threshold: req.Threshold,
		})
		if err != nil {
			return nil, err
		}
		return toAlert(row), nil
	})
}

func (s *Server) alertDelete(c *gin.Context) {
	var req idRequest
	s.handle(c, "stock:alert:delete", &req, func(context.Context) (any, error) {
		if req.ID == 0 {
			return nil, ncode.Fail(ncode.CodeBadRequest, "请提供盯盘编号")
		}
		return nil, s.svc.DeleteRule(req.ID)
	})
}

func (s *Server) alertEventSearch(c *gin.Context) {
	var req pageRequest
	s.handlePage(c, "stock:alert:view", &req, func(context.Context) (any, int, int, int64, error) {
		return s.eventPage(req.Code, req.Page, req.Size, ncode.Ascending(req.Sort))
	})
}

func (s *Server) monitorScan(c *gin.Context) {
	s.handle(c, "stock:monitor:execute", &emptyRequest{}, func(ctx context.Context) (any, error) {
		result, err := s.runner.Scan(ctx)
		if err != nil {
			return nil, err
		}
		return toScan(result), nil
	})
}

func (s *Server) monitorGet(c *gin.Context) {
	s.handle(c, "stock:monitor:view", &emptyRequest{}, func(context.Context) (any, error) {
		return s.monitorData(), nil
	})
}

func (s *Server) indicesData(ctx context.Context) ([]quoteDTO, error) {
	rows, err := s.svc.Indices(ctx)
	if err != nil {
		return nil, err
	}
	return toQuotes(rows), nil
}

func (s *Server) quotesData(ctx context.Context, refresh bool) (quoteListDTO, error) {
	list, err := s.svc.WatchQuotes(ctx, refresh)
	if err != nil {
		return quoteListDTO{}, err
	}
	return toWatchQuotes(list), nil
}

func (s *Server) quoteData(ctx context.Context, code string, refresh bool) (quoteDTO, error) {
	if code == "" {
		return quoteDTO{}, ncode.Fail(ncode.CodeBadRequest, "请提供证券代码")
	}
	row, err := s.svc.Quote(ctx, code, refresh)
	if err != nil {
		return quoteDTO{}, err
	}
	return toQuote(row), nil
}

func (s *Server) klineData(ctx context.Context, code string, size int, refresh bool) (klineDTO, error) {
	if code == "" {
		return klineDTO{}, ncode.Fail(ncode.CodeBadRequest, "请提供证券代码")
	}
	view, err := s.svc.Kline(ctx, code, size, refresh)
	if err != nil {
		return klineDTO{}, err
	}
	return toKline(view), nil
}

func (s *Server) watchData() ([]watchDTO, error) {
	rows, err := s.svc.ListWatch()
	if err != nil {
		return nil, err
	}
	return toWatches(rows), nil
}

func (s *Server) accountData(ctx context.Context) (accountDTO, error) {
	view, err := s.svc.Account(ctx)
	if err != nil {
		return accountDTO{}, err
	}
	return toAccount(view), nil
}

func (s *Server) strategyData() ([]strategyDTO, error) {
	rows, err := s.svc.Strategies()
	if err != nil {
		return nil, err
	}
	return toStrategies(rows), nil
}

func (s *Server) alertData() ([]alertDTO, error) {
	rows, err := s.svc.Rules()
	if err != nil {
		return nil, err
	}
	return toAlerts(rows), nil
}

func (s *Server) monitorData() monitorDTO {
	return toMonitor(s.runner.Status())
}

func (s *Server) orderPage(code string, page, size int, asc bool) ([]orderDTO, int, int, int64, error) {
	page, size = ncode.NormalizePage(page, size)
	rows, total, err := s.svc.PageOrders(code, page, size, asc)
	if err != nil {
		return nil, page, size, 0, err
	}
	return toOrders(rows), page, size, total, nil
}

func (s *Server) predictionPage(code string, page, size int, asc bool) ([]predictionDTO, int, int, int64, error) {
	page, size = ncode.NormalizePage(page, size)
	rows, total, err := s.svc.PagePredictions(code, page, size, asc)
	if err != nil {
		return nil, page, size, 0, err
	}
	return toPredictions(rows), page, size, total, nil
}

func (s *Server) eventPage(code string, page, size int, asc bool) ([]eventDTO, int, int, int64, error) {
	page, size = ncode.NormalizePage(page, size)
	rows, total, err := s.svc.PageEvents(code, page, size, asc)
	if err != nil {
		return nil, page, size, 0, err
	}
	return toEvents(rows), page, size, total, nil
}

func (s *Server) backtestPage(strategyID uint, page, size int, asc bool) ([]backtestDTO, int, int, int64, error) {
	page, size = ncode.NormalizePage(page, size)
	rows, total, err := s.svc.PageBacktests(strategyID, page, size, asc)
	if err != nil {
		return nil, page, size, 0, err
	}
	return toBacktests(rows), page, size, total, nil
}
