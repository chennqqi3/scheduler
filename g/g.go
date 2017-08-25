package g
 
import (
       "github-beta.huawei.com/hipaas/common/storage/node"
       "github-beta.huawei.com/hipaas/common/storage/realstate"
       "github.com/garyburd/redigo/redis"
       "log"
       "runtime"
)
 
func init() {
       runtime.GOMAXPROCS(runtime.NumCPU())
       log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
       MsgEngineStateChan = make(chan bool, 10)
}
 
var RealState ContainerRealState
var RealNodeState MinionStorageDriver
var RealMinionState ContainerRealState
 
type ContainerRealState realstate.ContainerRealState
type MinionStorageDriver node.MinionStorageDriver
 
// RedisConnPool save redis connections.
var RedisConnPool *redis.Pool
 
var MsgEngineStateChan chan bool
