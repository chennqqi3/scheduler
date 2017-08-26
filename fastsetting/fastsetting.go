package fastsetting
 
import (
       "encoding/json"
       "errors"
       "fmt"
       "sync"
       "time"
 
       "github.com/garyburd/redigo/redis"
 
       FS "github-beta.huawei.com/hipaas/common/fastsetting"
       ME "github-beta.huawei.com/hipaas/common/msgengine"
       "github-beta.huawei.com/hipaas/common/storage/realstate"
       "github-beta.huawei.com/hipaas/common/util/taskpool"
       "github-beta.huawei.com/hipaas/common/util/uuid"
       "github-beta.huawei.com/hipaas/glog"
       "github-beta.huawei.com/hipaas/scheduler/executor"
       "github-beta.huawei.com/hipaas/scheduler/g"
)

const (
       containerFastSettingTableName       = "hipaas_fastsetting_container_table"
       fastSettingJobQueue                 = "hipaas_fastsetting_job"
       fastSettingJobPrefix                = "hipaas_fastsetting_job_"
       task_timeout                  int64 = 600 // second
       task_interval                 int64 = 100 // millsecond
       health_check_interval         int   = 60
       sync_check_interval           int   = 600
       health_check_timeout          int64 = 40 * 60
       execErrorOccur                      = ""
)
 
type FastSetting interface {
       AddFastSettingJob(fastSetting *FS.FastSetting) (string, error)
       GetFastSettingResult(uuid string) ([]byte, error)
       GetAllCTNRExecMode() (map[string]*TaskStatus, error)
       GetCTNRExecMode(containerID string) (*TaskStatus, error)
       AddCTNRExecMode(appName string, containerID string, operateType string) error

DeleteCTNRExecMode(containerID string) error
       UpdateCtnrExecMode(appName string, containerID string, operateType string) error
       IsCTNRExecModeExist(containerId string) (bool, error)
}
 
type FastSettingMainDriver struct {
       jobQueue  *JobQueue
       storage   *RedisStorage
       task      taskpool.TaskPoolInterface
       realState realstate.ContainerRealState
}
 
type result struct {
       containerID string
       err         error
}
 
var errTimeout error = errors.New("fastsetting timeout")
 
func (this *FastSettingMainDriver) AddFastSettingJob(fastSetting *FS.FastSetting) (string, error) {
       // checkout appname-containerID
       switch fastSetting.OperateType {
       case FS.Maint:
case FS.Normal:
       case FS.Restart:
       default:
              return "", fmt.Errorf("undefined mode: %s", fastSetting.OperateType)
       }
       containerlist := this.realState.Containers(fastSetting.AppName)
       mapCtnrID := make(map[string]bool)
       for _, container := range containerlist {
              mapCtnrID[container.ID] = true
       }
       for _, ctnrStatus := range fastSetting.Containers {
              if _, exist := mapCtnrID[ctnrStatus.ID]; !exist {
                     return "", fmt.Errorf("ContainerID(%s) isn't matched appName(%s)", ctnrStatus.ID, fastSetting.AppName)
              }
       }
 
       return this.addFastSettingJob(fastSetting)
}

func (this *FastSettingMainDriver) GetFastSettingResult(uuid string) ([]byte, error) {
       return this.getFastSettingJob(fastSettingJobPrefix uuid, "result")
}
 
func (this *FastSettingMainDriver) handleCtnrExecMode() {
       duration := time.Duration(health_check_interval) * time.Second
       for {
              time.Sleep(duration)
              mapCtnrsFastSetting, err := this.GetAllCTNRExecMode()
              if nil != err {
                     glog.Error(err)
                     continue
              }
              for id, ts := range mapCtnrsFastSetting {
                     if ts.Birthday == forever {
                            continue
                     } else {
                            if time.Now().Unix()-ts.Birthday > health_check_timeout {
							if err := this.DeleteCTNRExecMode(id); nil != err {
                                          glog.Error("DeleteCTNRExecMode failed, Error:", err)
                                          continue
                                   }
                                   glog.Warning("fastsetting task timeout, drop the task: ", ts)
                            }
                     }
              }
       }
}
 
func (this *FastSettingMainDriver) SyncAllappExecMode() {
       duration := time.Duration(sync_check_interval) * time.Second
       for {
              time.Sleep(duration)
              mapCtnrsFastSetting, err := this.GetAllCTNRExecMode()
              if nil != err {
                     glog.Errorf("GetAllCTNRExecMode failed:%v", err)
                     continue
              }
			  for _, ts := range mapCtnrsFastSetting {
                     if ts.Appname == "" {
                            glog.Errorf("sync fastsetting mode failed:appname is null, ctrnId:%s", ts.ID)
                            continue
                     }
                     if ts.ID == "" {
                            glog.Errorf("sync fastsetting mode failed:ctrnId is null, appname:%s", ts.Appname)
                            continue
                     }
 
                     bExist, err := g.RealState.IsContainerExist(ts.Appname, ts.ID)
                     if nil != err {
                            glog.Errorf("IsContainerExist appname:%s, containerid:%s failed:%v", ts.Appname, ts.ID, err)
                            continue
                     }
					 if !bExist {
                            if err := this.DeleteCTNRExecMode(ts.ID); nil != err {
                                   glog.Errorf("sync deleteCTNRExecMode appname:%s id:%s failed:%v", ts.Appname, ts.ID, err)
                            } else {
                                   glog.Infof("sync deleteCTNRExecMode appname:%s id:%s success", ts.Appname, ts.ID)
                            }
                     }
              }
       }
}
 
func (this *FastSettingMainDriver) ApplyOneJob() (*Job, error) {
       rc := this.storage.connPool.Get()
       defer rc.Close()
       job, err := this.jobQueue.Dequeue(rc, fastSettingJobQueue)
       if nil != err {
              return nil, err
       }
       if nil == job {
              return nil, nil
       }
	   fastSettingJob := fastSettingJobPrefix + job.JobID
       args := []interface{}{fastSettingJob, task_timeout}
       if _, err := rc.Do("EXPIRE", args...); nil != err {
              glog.Errorf("set fastsetting job timeout failed. Drop the job:{%v}, Error:%v.", fastSettingJob, err)
              return nil, fmt.Errorf("[ERROR] EXPIRE %v fail: %v", args, err)
       }
 
       return job, nil
 
}

func (this *FastSettingMainDriver) HandleFastSetting() {
       duration := time.Duration(1) * time.Second
       for {
              time.Sleep(duration)
              // maintenance mode
              if g.Config().MAINT.Switch {
                     continue
              }
              for {
                     job, err := this.ApplyOneJob()
                     if nil != err {
                            glog.Error(err)
                            continue
                     }
					 if nil == job {
                            break
                     }
                     this.task.Require()
                     go func(job *Job) {
                            defer this.task.Release()
                            this.handleFastSetting(job)
                     }(job)
              }
       }
}
 
func (this *FastSettingMainDriver) handleFastSetting(job *Job) {
       glog.Infof("begin handle fast setting job, jobid: %s, content: %v ", job.JobID, *((*job).JobContent))
       fastSetting := job.JobContent
       ctnrsCount := len(fastSetting.Containers)
       chanFastSettingJobFinish := make(chan result, ctnrsCount)
 
       for i := 0; i < ctnrsCount; i++ {
              go this.doTheJob(fastSetting, i, chanFastSettingJobFinish)
       }
       successctnr := make(map[string]error, 0)
       duration := time.Duration(task_timeout) * time.Second
	   interval := time.Duration(task_interval) * time.Millisecond
       now := time.Now()
       for {
              time.Sleep(interval)
              if duration <= time.Since(now) {
                     glog.Error("handle fastsetting job time-out. Job: ", *job)
                     break
              }
              select {
              case r := <-chanFastSettingJobFinish:
                     successctnr[r.containerID] = r.err
                     // send fastsetting success message immediately
                     if r.err == nil {
                            ME.NewEventReporter(ME.FastSettingSuccess, ME.FastSettingSuccessData{
                                   AppName:     fastSetting.AppName,
                                   Birthday:    time.Now(),
                                   OperateType: fastSetting.OperateType,
								   ContainerID: r.containerID,
                                   Message:     fmt.Sprintf("Finish FastSetting Job %v --> app: %v, container: %v", fastSetting.OperateType, fastSetting.AppName, r.containerID),
                            })
                     }
              default:
              }
 
              if ctnrsCount == len(successctnr) {
                     break
              }
       }
 
       for index, ctnr := range fastSetting.Containers {
              err, exist := successctnr[ctnr.ID]
              if exist == false {
                     err = errTimeout
              }
              if err != nil {
                     fastSetting.Containers[index].Status = FS.ContainerStatusFail
                     glog.Errorf("do fastsetting for container %s error: %s", ctnr.ID, err.Error())
                     ME.NewEventReporter(ME.ExecResult, ME.ExecResultData{
                            Reason:   "fastSetting fail",
							AppName:  fastSetting.AppName,
                            Error:    err.Error(),
                            Birthday: time.Now(),
                            Message:  fmt.Sprintf("FastSetting Job %v Fail --> app: %v, container: %v, err: %v", fastSetting.OperateType, fastSetting.AppName, fastSetting.Containers[index], err),
                     })
              } else {
                     fastSetting.Containers[index].Status = FS.ContainerStatusSuccess
              }
       }
 
       if err := this.setFastSettingJob(fastSettingJobPrefix+job.JobID, fastSetting, "result"); nil != err {
              glog.Errorf("execut fastsetting job failed. Error: %v, drop this job.", err)
              return
       }
       glog.Infof("finish handle job, jobid: %s, content: %v ", job.JobID, *((*job).JobContent))
}

func (this *FastSettingMainDriver) doTheJob(fastSetting *FS.FastSetting, index int, resultChan chan result) {
       ctnrID := fastSetting.Containers[index].ID
 
       ts, err := this.GetCTNRExecMode(ctnrID)
       if nil != err {
              resultChan <- result{ctnrID, err}
              return
       }
       ctnr := this.realState.GetContainersMap(fastSetting.AppName)[ctnrID]
       if nil == ctnr {
              resultChan <- result{ctnrID, errors.New("container does not exist")}
              return
       }
 
       switch fastSetting.OperateType {
       case FS.Normal:
              if nil == ts {
                     resultChan <- result{ctnr.ID, nil}
                     return
              }
			  if ts.OperType != FS.Maint {
                     resultChan <- result{ctnr.ID, fmt.Errorf("fastsetting from %s to %s", ts.OperType, fastSetting.OperateType)}
                     return
              }
 
              afterDockerExec := func(err error) {
                     defer func() { resultChan <- result{ctnr.ID, err} }()
 
                     if err != nil {
                            return
                     }
                     err = this.DeleteCTNRExecMode(ctnr.ID)
                     // this.UpdateCtnrExecMode(ctnr.ID, fastSetting.OperateType)
              }
              executor.GetDockerManager().DockerExecAsync(executor.Restart, ctnr, func() error { return nil }, afterDockerExec)
 
       case FS.Maint:
              if nil != ts {
                     if ts.OperType == FS.Maint {
                            glog.Infof("fastsetting success because job already exists, container id: %s, type: %s", ctnr.ID, fastSetting.OperateType)
							resultChan <- result{ctnr.ID, nil}
                     } else {
                            resultChan <- result{ctnr.ID, errors.New("fastsetting job already exists")}
                     }
                     return
              }
 
              beforeDockerExec := func() error {
                     err := this.AddCTNRExecMode(ctnr.AppName, ctnr.ID, fastSetting.OperateType)
                     if err != nil {
                            resultChan <- result{ctnr.ID, err}
                     }
                     return err
              }
              afterDockerExec := func(err error) {
                     defer func() { resultChan <- result{ctnr.ID, err} }()
 
                     if err != nil {
                            this.DeleteCTNRExecMode(ctnr.ID)
                            return
                     }
                     this.UpdateCtnrExecMode(ctnr.AppName, ctnr.ID, fastSetting.OperateType)
              }
			  executor.GetDockerManager().DockerExecAsync(executor.Stop, ctnr, beforeDockerExec, afterDockerExec)
 
       case FS.Restart:
              beforeDockerExec := func() error {
                     err := this.AddCTNRExecMode(ctnr.AppName, ctnr.ID, fastSetting.OperateType)
                     if err != nil {
                            resultChan <- result{ctnr.ID, err}
                     }
                     return err
              }
              afterDockerExec := func(err error) {
                     defer func() { resultChan <- result{ctnr.ID, err} }()
 
                     if err != nil {
                            return
                     }
                     err = this.DeleteCTNRExecMode(ctnr.ID)
                     // this.UpdateCtnrExecMode(ctnr.ID, fastSetting.OperateType)
              }
			  executor.GetDockerManager().DockerExecAsync(executor.Restart, ctnr, beforeDockerExec, afterDockerExec)
 
       default:
              resultChan <- result{ctnr.ID, fmt.Errorf("undefined mode: %s", fastSetting.OperateType)}
       }
}
 
func (this *FastSettingMainDriver) addFastSettingJob(fastSetting *FS.FastSetting) (string, error) {
       uuid := uuid.GetGuid()
       err := this.setFastSettingJob(fastSettingJobPrefix+uuid, fastSetting, "source")
       if err != nil {
              return "", err
       }
 
       job := &Job{
              JobID:      uuid,
              JobContent: fastSetting,
       }
       rc := this.storage.connPool.Get()
       defer rc.Close()
	   err = this.jobQueue.Enqueue(rc, fastSettingJobQueue, job)
       if err != nil {
              return "", err
       }
       return uuid, nil
}
 
func (this *FastSettingMainDriver) setFastSettingJob(keyName string, fastSetting *FS.FastSetting, dataType string) error {
       jsonData, err := json.Marshal(fastSetting)
       if err != nil {
              glog.Error("Marshal fastsetting.FastSetting fail:", err)
              return err
       }
       rc := this.storage.connPool.Get()
       defer rc.Close()
       args := []interface{}{keyName, dataType, jsonData}
       _, err = rc.Do("HMSET", args...)
       if err != nil {
              glog.Errorf("HMSET %v fail: %v", args, err)
              return err
       }
	   args = []interface{}{keyName, task_timeout}
       if _, err := rc.Do("EXPIRE", args...); nil != err {
              glog.Errorf("set fastsetting job timeout failed:job:%v, Error:%v.", keyName, err)
              return err
       }
       return nil
}
 
func (this *FastSettingMainDriver) getFastSettingJob(keyName string, dataType string) ([]byte, error) {
       rc := this.storage.connPool.Get()
       defer rc.Close()
       jsonData, err := redis.Strings(rc.Do("HMGET", keyName, dataType))
       if err != nil {
              glog.Errorf("HMGET %s %s fail: %v", keyName, dataType, err)
              return nil, err
       }
       if len(jsonData) == 0 || jsonData[0] == "" {
              return nil, fmt.Errorf("keyname(%s) can't get result information", keyName)
       }
       return []byte(jsonData[0]), nil
}

func (this *FastSettingMainDriver) GetAllCTNRExecMode() (map[string]*TaskStatus, error) {
       return this.storage.GetAllCTNRExecMode()
}
 
func (this *FastSettingMainDriver) GetCTNRExecMode(containerID string) (*TaskStatus, error) {
       return this.storage.GetCTNRExecMode(containerID)
}
 
func (this *FastSettingMainDriver) AddCTNRExecMode(appName string, containerID string, operateType string) error {
       err := this.storage.AddCTNRExecMode(appName, containerID, operateType)
       if err != nil {
              return err
       }
 
       return nil
}

func (this *FastSettingMainDriver) DeleteCTNRExecMode(containerID string) error {
       err := this.storage.DeleteCTNRExecMode(containerID)
       if err != nil {
              return err
       }
 
       return nil
}
 
func (this *FastSettingMainDriver) UpdateCtnrExecMode(appName string, containerID string, operateType string) error {
       err := this.storage.UpdateCtnrExecMode(appName, containerID, operateType)
       if err != nil {
              return err
       }
       return nil
}
 
func (this *FastSettingMainDriver) IsCTNRExecModeExist(containerId string) (bool, error) {
       return this.storage.IsCTNRExecModeExist(containerId)
	   }
	   type RedisStorage struct {
       connPool *redis.Pool
}
 
const forever int64 = -1
 
type TaskStatus struct {
       Appname  string `json:"appname"`
       ID       string `json:"id"`
       Birthday int64  `json:"birthday"`
       OperType string `json:"opertype"`
}
 
func (this *RedisStorage) GetAllCTNRExecMode() (map[string]*TaskStatus, error) {
       mapCTNRExecMode := make(map[string]*TaskStatus)
 
       rc := this.connPool.Get()
       defer rc.Close()
 
       data, err := redis.Strings(rc.Do("HGETALL", containerFastSettingTableName))
	   if err != nil {
              return mapCTNRExecMode, err
       }
       if len(data) == 0 {
              return mapCTNRExecMode, nil
       }
       for i := 0; i < len(data); i = i   2 {
              ts := TaskStatus{}
              if err := json.Unmarshal([]byte(data[i 1]), &ts); nil != err {
                     return mapCTNRExecMode, err
              }
              mapCTNRExecMode[data[i]] = &ts
       }
 
       return mapCTNRExecMode, nil
}
 
func (this *RedisStorage) GetCTNRExecMode(containerID string) (*TaskStatus, error) {
       rc := this.connPool.Get()
       defer rc.Close()
 
       exist, err := redis.Int(rc.Do("HEXISTS", containerFastSettingTableName, containerID))
       if err != nil {
              return nil, err
       }
	   if 0 == exist {
              return nil, nil
       }
 
       data, err := redis.String(rc.Do("HGET", containerFastSettingTableName, containerID))
       if err != nil {
              return nil, err
       }
 
       ts := TaskStatus{}
       if err := json.Unmarshal([]byte(data), &ts); nil != err {
              return nil, err
       }
 
       return &ts, nil
}
 
func (this *RedisStorage) AddCTNRExecMode(appName string, containerID string, operateType string) error {
       rc := this.connPool.Get()
       defer rc.Close()
 
       ts := TaskStatus{
              Appname:  appName,
              ID:       containerID,
              Birthday: time.Now().Unix(),
              OperType: operateType,
       }
	   tsbyte, err := json.Marshal(ts)
       if nil != err {
              return err
       }
 
       _, err = rc.Do("HMSET", containerFastSettingTableName, containerID, string(tsbyte))
       if err != nil {
              glog.Errorf("HMSET %s fail: %v", tsbyte, err)
              return err
       }
 
       return nil
}
 
func (this *RedisStorage) DeleteCTNRExecMode(containerID string) error {
       rc := this.connPool.Get()
       defer rc.Close()
 
       args := []interface{}{containerFastSettingTableName, containerID}
       _, err := rc.Do("HDEL", args...)
       if err != nil {
              glog.Errorf("HDEL %v fail: %v", args, err)
              return err
       }
	   return nil
}
 
func (this *RedisStorage) UpdateCtnrExecMode(appName string, containerID string, operateType string) error {
       rc := this.connPool.Get()
       defer rc.Close()
 
       ts := TaskStatus{
              Appname:  appName,
              ID:       containerID,
              Birthday: forever, // do not kill this container.
              OperType: operateType,
       }
 
       tsbyte, err := json.Marshal(ts)
       if nil != err {
              return err
       }
       _, err = rc.Do("HMSET", containerFastSettingTableName, containerID, string(tsbyte))
       if err != nil {
              glog.Errorf("HMSET %s fail: %v", tsbyte, err)
              return err
       }
 
       return nil
}

func (this *RedisStorage) IsCTNRExecModeExist(containerId string) (bool, error) {
       rc := this.connPool.Get()
       defer rc.Close()
 
       exist, err := redis.Int(rc.Do("HEXISTS", containerFastSettingTableName, containerId))
       if err != nil {
              return false, err
       }
 
       if 1 == exist {
              return true, nil
       } else {
              return false, nil
       }
}
 
var fastSettingMainDriver *FastSettingMainDriver
var once sync.Once
func NewFastsetting(redisPool *redis.Pool, realState realstate.ContainerRealState) FastSetting {
       once.Do(func() {
              tp, _ := taskpool.NewTaskPool(100)
              fastSettingMainDriver = &FastSettingMainDriver{
                     storage:   &RedisStorage{connPool: redisPool},
                     jobQueue:  new(JobQueue),
                     realState: realState,
                     task:      tp,
              }
              go fastSettingMainDriver.HandleFastSetting()
              go fastSettingMainDriver.handleCtnrExecMode()
              go fastSettingMainDriver.SyncAllappExecMode()
       })
       return fastSettingMainDriver
}

func GetFastsetting() (FastSetting, error) {
       if fastSettingMainDriver == nil {
              return nil, errors.New("fastSettingMainDriver not init!")
       }
       return fastSettingMainDriver, nil
}
