package ingest

import "context"

// LocationPublisher abstracts publishing driver location events.
type LocationPublisher interface {
	Publish(ctx context.Context, loc Location) error
}

// IngestService receives location updates and forwards them to a publisher.
type IngestService struct {
	publisher LocationPublisher
}

// NewIngestService constructs an IngestService with the given publisher.
func NewIngestService(publisher LocationPublisher) *IngestService {
	return &IngestService{publisher: publisher}
}

// Ingest forwards the location to the configured publisher.
func (s *IngestService) Ingest(ctx context.Context, loc Location) error {
	return s.publisher.Publish(ctx, loc)
}
