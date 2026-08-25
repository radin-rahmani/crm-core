package delivery

import (
	"crm-core/internal/domain"
	"crm-core/internal/worker"
	pb "crm-core/proto"
	"io"
	"time"
)

type GRPCHandler struct {
	pb.UnimplementedLoggerServiceServer
	workerPool *worker.LogWorkerPool
}

func NewGRPCHandler(wp *worker.LogWorkerPool) *GRPCHandler {
	return &GRPCHandler{workerPool: wp}
}

func (h *GRPCHandler) StreamLogs(stream pb.LoggerService_StreamLogsServer) error {
	var count int32

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&pb.LogResponse{
				ProcessedCount: count,
				Success:        true,
			})
		}
		if err != nil {
			return err
		}

		h.workerPool.Enqueue(domain.LogEntry{
			UserID:    req.UserId,
			Action:    req.Action,
			Details:   req.Details,
			CreatedAt: time.Unix(req.Timestamp, 0),
		})
		count++
	}
}
