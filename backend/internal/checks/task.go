package checks

// TaskName marks a raw Lambda payload as a check-pipeline task.
const TaskName = "runCheck"

// Task is the async payload that tells the worker which check to run. OwnerID rides along so the
// worker can refuse a task whose check belongs to someone else.
type Task struct {
	Task    string `json:"vaultTask"`
	CheckID string `json:"checkId"`
	OwnerID string `json:"ownerId"`
}

// NewTask builds the payload for one check.
func NewTask(checkID string, ownerID string) Task {
	return Task{Task: TaskName, CheckID: checkID, OwnerID: ownerID}
}
