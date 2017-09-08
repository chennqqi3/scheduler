package fastsetting
 
import (
       "bytes"
       "encoding/json"
       "errors"
       "github-beta.huawei.com/hipaas/common/fastsetting"
       "github.com/garyburd/redigo/redis"
       "sync"
)
 
type Job struct {
       JobID      string                   `json:"id"`
       JobContent *fastsetting.FastSetting `json:"content"`
}
 
type JobQueue struct {
       sync.RWMutex
}
 
// Enqueue: Add a job
func (this *JobQueue) Enqueue(rc redis.Conn, queueName string, job *Job) error {
       jobData, err := json.Marshal(job)
       if err != nil {
              return err
       }
 
       this.Lock()
       defer this.Unlock()
 
       _, err = rc.Do("RPUSH", queueName, jobData)
       if err != nil {
              return err
       }
 
       return nil
}
 
// Dequeue: Get a job
func (this *JobQueue) Dequeue(rc redis.Conn, queueName string) (*Job, error) {
       count, err := this.QueuedJobCount(rc, queueName)
       if err != nil {
              return nil, err
       }
       if count == 0 {
              return nil, nil
       }
 
       this.Lock()
       defer this.Unlock()
 
       jobData, err := rc.Do("LPOP", queueName)
       if err != nil {
              return nil, err
       }
 
       var job Job
       decoder := json.NewDecoder(bytes.NewReader(jobData.([]byte)))
       if err := decoder.Decode(&job); err != nil {
              return nil, err
       }
 
       return &job, nil
}
 
func (this *JobQueue) QueuedJobCount(rc redis.Conn, queueName string) (int, error) {
       this.RLock()
       defer this.RUnlock()
 
       lenQueue, err := rc.Do("LLEN", queueName)
       if err != nil {
              return 0, err
       }
 
       count, ok := lenQueue.(int64)
       if !ok {
              return 0, errors.New("type conversion error")
       }
       return int(count), nil
}
