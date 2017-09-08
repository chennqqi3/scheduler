package g
 
import (
       "fmt"
       "strings"
       "sync"
       "time"
 
       "github-beta.huawei.com/hipaas/common/storage/realstate"
       "github-beta.huawei.com/hipaas/glog"
       "github.com/garyburd/redigo/redis"
)
 
const (
       healthRedisKey = "hipaas_health_check_record"
       fieldSeparator = "/"
)
 
var HealthCheckTries healthCheckTries
 
type healthCheckTries struct {
       once sync.Once
}
 
func (h *healthCheckTries) Increase(ctnr *realstate.Container) (int, error) {
       h.once.Do(h.totalUpdate)
 
       if ctnr == nil {
              return 0, fmt.Errorf("container cannot be nil")
       }
 
       conn := RedisConnPool.Get()
       defer conn.Close()
       if err := conn.Err(); err != nil {
              return 0, fmt.Errorf("get redis conn failed: %s", err.Error())
       }
 
       field := h.generateField(ctnr)
       tries, err := redis.Int(conn.Do("HINCRBY", healthRedisKey, field, 1))
       if err != nil {
              return 0, fmt.Errorf("increase health check tries for %s error: %s", field, err.Error())
       }
       return tries, nil
}
 
func (h *healthCheckTries) Delete(ctnr *realstate.Container) error {
       h.once.Do(h.totalUpdate)
 
       if ctnr == nil {
              return fmt.Errorf("container cannot be nil")
       }
       return h.remove(h.generateField(ctnr))
}
 
func (h *healthCheckTries) remove(field string) error {
       conn := RedisConnPool.Get()
       defer conn.Close()
       if err := conn.Err(); err != nil {
              return fmt.Errorf("get redis conn failed: %s", err.Error())
       }
 
       _, err := conn.Do("HDEL", healthRedisKey, field)
       if err != nil {
              return fmt.Errorf("delete health check tries for %s error: %s", field, err.Error())
       }
       return nil
}
 
func (h *healthCheckTries) generateField(ctnr *realstate.Container) string {
       if ctnr == nil {
              return ""
       }
       return ctnr.AppName + fieldSeparator + ctnr.ID
}
 
func (h *healthCheckTries) totalUpdate() {
       go func() {
              duration := time.Minute * 10
              for {
                     time.Sleep(duration)
                     h.realTotalUpdate()
              }
       }()
}
 
func (h *healthCheckTries) realTotalUpdate() {
       conn := RedisConnPool.Get()
       defer conn.Close()
       if err := conn.Err(); err != nil {
              glog.Errorf("get redis conn failed: %s", err.Error())
              return
       }
 
       fields, err := redis.Strings(conn.Do("HKEYS", healthRedisKey))
       if err != nil {
              glog.Errorf("get all fields error: %s", err.Error())
              return
       }
 
       for _, field := range fields {
              sField := strings.Split(field, fieldSeparator)
              if len(sField) != 2 {
                     glog.Errorf("field %s is not in correct format", field)
                     continue
              }
              appName, ctnrID := sField[0], sField[1]
              containers := RealState.GetContainersMap(appName)
              if _, ok := containers[ctnrID]; ok == false {
                     if err := h.remove(field); err != nil {
                            glog.Error(err.Error())
                     }
              }
       }
}
