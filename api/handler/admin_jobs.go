package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"

	"convertpdfgo/pkg/queue"
)

// allowedQueue rejects path traversal and unknown queues. Asynq accepts
// any string here, so we whitelist to the three queues we actually use.
func allowedQueue(name string) bool {
	switch name {
	case queue.QueueCritical, queue.QueueDefault, queue.QueueLow:
		return true
	}
	return false
}

// jobInfo is the shape we serialise to the admin UI. Asynq's TaskInfo has
// a lot of internal fields we don't want to leak (raw payload bytes etc.)
// so we project to a stable, human-friendly subset.
type jobInfo struct {
	ID         string    `json:"id"`
	Type       string    `json:"type"`
	Queue      string    `json:"queue"`
	State      string    `json:"state"`
	Retried    int       `json:"retried"`
	MaxRetry   int       `json:"max_retry"`
	LastErr    string    `json:"last_err,omitempty"`
	NextRetry  time.Time `json:"next_retry,omitempty"`
	EnqueuedAt time.Time `json:"enqueued_at,omitempty"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
}

func toJobInfo(t *asynq.TaskInfo) jobInfo {
	return jobInfo{
		ID:          t.ID,
		Type:        t.Type,
		Queue:       t.Queue,
		State:       t.State.String(),
		Retried:     t.Retried,
		MaxRetry:    t.MaxRetry,
		LastErr:     t.LastErr,
		NextRetry:   t.NextProcessAt,
		EnqueuedAt:  t.LastFailedAt,
		CompletedAt: t.CompletedAt,
	}
}

// ListJobsAdmin godoc
// @Router       /admin/jobs/{queue}/{state} [get]
// @Security     ApiKeyAuth
// @Summary      List jobs in a given queue+state (admin)
// @Tags         admin-jobs
// @Param        queue path  string true  "queue name (critical | default | low)"
// @Param        state path  string true  "pending | active | scheduled | retry | archived | completed"
// @Param        page  query int    false "page index (1-based, default 1)"
// @Param        size  query int    false "page size (default 50, max 200)"
// @Success      200 {object} models.Response
func (h Handler) ListJobsAdmin(c *gin.Context) {
	q := c.Param("queue")
	state := c.Param("state")
	if !allowedQueue(q) {
		handleResponse(c, h.log, "unknown queue", http.StatusBadRequest, q)
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(c.DefaultQuery("size", "50"))
	if size <= 0 || size > 200 {
		size = 50
	}
	opt := asynq.PageSize(size)
	pageOpt := asynq.Page(page)

	inspector := asynq.NewInspector(queue.RedisOpt(h.cfg))
	defer inspector.Close()

	var (
		tasks []*asynq.TaskInfo
		err   error
	)
	switch state {
	case "pending":
		tasks, err = inspector.ListPendingTasks(q, opt, pageOpt)
	case "active":
		tasks, err = inspector.ListActiveTasks(q, opt, pageOpt)
	case "scheduled":
		tasks, err = inspector.ListScheduledTasks(q, opt, pageOpt)
	case "retry":
		tasks, err = inspector.ListRetryTasks(q, opt, pageOpt)
	case "archived":
		tasks, err = inspector.ListArchivedTasks(q, opt, pageOpt)
	case "completed":
		tasks, err = inspector.ListCompletedTasks(q, opt, pageOpt)
	default:
		handleResponse(c, h.log, "unknown state", http.StatusBadRequest, state)
		return
	}
	if err != nil {
		handleResponse(c, h.log, "inspector error", http.StatusInternalServerError, err.Error())
		return
	}

	out := make([]jobInfo, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, toJobInfo(t))
	}
	handleResponse(c, h.log, "jobs listed", http.StatusOK, gin.H{
		"items": out,
		"queue": q,
		"state": state,
		"page":  page,
		"size":  size,
	})
}

// RetryJobAdmin godoc
// @Router       /admin/jobs/{queue}/{id}/retry [post]
// @Security     ApiKeyAuth
// @Summary      Re-enqueue an archived/retry/scheduled task immediately (admin)
// @Tags         admin-jobs
// @Param        queue path string true "queue name"
// @Param        id    path string true "task id"
// @Success      200 {object} models.Response
func (h Handler) RetryJobAdmin(c *gin.Context) {
	q := c.Param("queue")
	id := c.Param("id")
	if !allowedQueue(q) {
		handleResponse(c, h.log, "unknown queue", http.StatusBadRequest, q)
		return
	}

	inspector := asynq.NewInspector(queue.RedisOpt(h.cfg))
	defer inspector.Close()

	if err := inspector.RunTask(q, id); err != nil {
		handleResponse(c, h.log, "retry failed", http.StatusInternalServerError, err.Error())
		return
	}
	h.audit(c, "admin_job_retry", c.GetString("user_id"), id, q)
	handleResponse(c, h.log, "task re-enqueued", http.StatusOK, gin.H{"id": id, "queue": q})
}

// DeleteJobAdmin godoc
// @Router       /admin/jobs/{queue}/{id} [delete]
// @Security     ApiKeyAuth
// @Summary      Delete a queued task (admin)
// @Tags         admin-jobs
// @Param        queue path string true "queue name"
// @Param        id    path string true "task id"
// @Success      200 {object} models.Response
func (h Handler) DeleteJobAdmin(c *gin.Context) {
	q := c.Param("queue")
	id := c.Param("id")
	if !allowedQueue(q) {
		handleResponse(c, h.log, "unknown queue", http.StatusBadRequest, q)
		return
	}

	inspector := asynq.NewInspector(queue.RedisOpt(h.cfg))
	defer inspector.Close()

	if err := inspector.DeleteTask(q, id); err != nil {
		handleResponse(c, h.log, "delete failed", http.StatusInternalServerError, err.Error())
		return
	}
	h.audit(c, "admin_job_delete", c.GetString("user_id"), id, q)
	handleResponse(c, h.log, "task deleted", http.StatusOK, gin.H{"id": id, "queue": q})
}

// ArchiveJobAdmin godoc — moves a pending/retry task to "archived" so it
// stops being retried. Useful for a flapping job you want to keep around
// for postmortem but never run again.
// @Router       /admin/jobs/{queue}/{id}/archive [post]
// @Security     ApiKeyAuth
// @Tags         admin-jobs
func (h Handler) ArchiveJobAdmin(c *gin.Context) {
	q := c.Param("queue")
	id := c.Param("id")
	if !allowedQueue(q) {
		handleResponse(c, h.log, "unknown queue", http.StatusBadRequest, q)
		return
	}

	inspector := asynq.NewInspector(queue.RedisOpt(h.cfg))
	defer inspector.Close()

	if err := inspector.ArchiveTask(q, id); err != nil {
		handleResponse(c, h.log, "archive failed", http.StatusInternalServerError, err.Error())
		return
	}
	h.audit(c, "admin_job_archive", c.GetString("user_id"), id, q)
	handleResponse(c, h.log, "task archived", http.StatusOK, gin.H{"id": id, "queue": q})
}
