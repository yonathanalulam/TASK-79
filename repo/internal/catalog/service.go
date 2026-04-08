package catalog

import (
	"context"
	"fmt"

	"fleetcommerce/internal/audit"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	repo     *Repository
	pool     *pgxpool.Pool
	auditSvc *audit.Service
}

func NewService(repo *Repository, pool *pgxpool.Pool, auditSvc *audit.Service) *Service {
	return &Service{repo: repo, pool: pool, auditSvc: auditSvc}
}

func (s *Service) ListBrands(ctx context.Context) ([]Brand, error) {
	return s.repo.ListBrands(ctx)
}

func (s *Service) ListSeries(ctx context.Context, brandID int) ([]Series, error) {
	return s.repo.ListSeries(ctx, brandID)
}

func (s *Service) ListModels(ctx context.Context, p ListParams) ([]VehicleModel, int, error) {
	return s.repo.ListModels(ctx, p)
}

func (s *Service) GetModel(ctx context.Context, id int) (*VehicleModel, error) {
	return s.repo.GetModel(ctx, id)
}

func (s *Service) CreateModel(ctx context.Context, p CreateModelParams, actorID int) (int, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	id, err := s.repo.CreateModel(ctx, tx, p)
	if err != nil {
		return 0, fmt.Errorf("create model: %w", err)
	}

	// Create initial version
	nextVer, _ := s.repo.GetNextVersionNumber(ctx, id)
	_, err = s.repo.CreateVersion(ctx, tx, VehicleModelVersion{
		VehicleModelID: id,
		VersionNumber:  nextVer,
		ModelName:      p.ModelName,
		Year:           p.Year,
		Description:    p.Description,
		StockQuantity:  p.StockQuantity,
		ExpiryDate:     p.ExpiryDate,
		Status:         "draft",
		IsCurrentDraft: true,
		CreatedBy:      &actorID,
	})
	if err != nil {
		return 0, fmt.Errorf("create version: %w", err)
	}

	s.auditSvc.LogTx(ctx, tx, audit.LogParams{
		EntityType:  "vehicle_model",
		EntityID:    id,
		Action:      "created",
		ActorUserID: &actorID,
		After:       p,
	})

	return id, tx.Commit(ctx)
}

func (s *Service) UpdateDraft(ctx context.Context, modelID int, p UpdateDraftParams, actorID int) error {
	before, err := s.repo.GetModel(ctx, modelID)
	if err != nil {
		return err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := s.repo.UpdateDraft(ctx, tx, modelID, p); err != nil {
		return err
	}

	nextVer, _ := s.repo.GetNextVersionNumber(ctx, modelID)
	_, err = s.repo.CreateVersion(ctx, tx, VehicleModelVersion{
		VehicleModelID: modelID,
		VersionNumber:  nextVer,
		ModelName:      p.ModelName,
		Year:           p.Year,
		Description:    p.Description,
		StockQuantity:  p.StockQuantity,
		ExpiryDate:     p.ExpiryDate,
		Status:         "draft",
		IsCurrentDraft: true,
		CreatedBy:      &actorID,
	})
	if err != nil {
		return fmt.Errorf("create version: %w", err)
	}

	s.auditSvc.LogTx(ctx, tx, audit.LogParams{
		EntityType:  "vehicle_model",
		EntityID:    modelID,
		Action:      "draft_updated",
		ActorUserID: &actorID,
		Before:      before,
		After:       p,
	})

	return tx.Commit(ctx)
}

func (s *Service) Publish(ctx context.Context, modelID int, actorID int) error {
	before, _ := s.repo.GetModel(ctx, modelID)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := s.repo.SetPublicationStatus(ctx, tx, modelID, "published"); err != nil {
		return err
	}

	beforeStatus := ""
	if before != nil {
		beforeStatus = before.PublicationStatus
	}
	s.auditSvc.LogTx(ctx, tx, audit.LogParams{
		EntityType:  "vehicle_model",
		EntityID:    modelID,
		Action:      "published",
		ActorUserID: &actorID,
		Before:      map[string]string{"publication_status": beforeStatus},
		After:       map[string]string{"publication_status": "published"},
	})

	return tx.Commit(ctx)
}

func (s *Service) Unpublish(ctx context.Context, modelID int, actorID int) error {
	before, _ := s.repo.GetModel(ctx, modelID)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := s.repo.SetPublicationStatus(ctx, tx, modelID, "unpublished"); err != nil {
		return err
	}

	beforeStatus := ""
	if before != nil {
		beforeStatus = before.PublicationStatus
	}
	s.auditSvc.LogTx(ctx, tx, audit.LogParams{
		EntityType:  "vehicle_model",
		EntityID:    modelID,
		Action:      "unpublished",
		ActorUserID: &actorID,
		Before:      map[string]string{"publication_status": beforeStatus},
		After:       map[string]string{"publication_status": "unpublished"},
	})

	return tx.Commit(ctx)
}

func (s *Service) ListVersions(ctx context.Context, modelID int) ([]VehicleModelVersion, error) {
	return s.repo.ListVersions(ctx, modelID)
}

func (s *Service) ListMedia(ctx context.Context, modelID int) ([]Media, error) {
	return s.repo.ListMedia(ctx, modelID)
}

func (s *Service) CreateMedia(ctx context.Context, m Media) (int, error) {
	return s.repo.CreateMedia(ctx, m)
}

func (s *Service) GetModelByCode(ctx context.Context, code string) (*VehicleModel, error) {
	return s.repo.GetModelByCode(ctx, code)
}

func (s *Service) Repo() *Repository {
	return s.repo
}
