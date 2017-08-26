package main
 
import (
       // "github-beta.huawei.com/hipaas/glog"
       "fmt"
       "github-beta.huawei.com/hipaas/scheduler/g"
       "runtime"
)
 
type SchedulerVersion struct {
       Version              string
       MinionMinimumVersion string
       GitCommit            string
       GoVersion            string
       Os                   string
       Arch                 string
       BuildTime            string
}
 
var version *SchedulerVersion
 
func init() {
       version = &SchedulerVersion{
              Version:              g.SCHEDULER_VERSION,
              MinionMinimumVersion: g.MINION_MINIMUM_VERSION,
              GitCommit:            GitCommit,
              GoVersion:            runtime.Version(),
              Os:                   runtime.GOOS,
              Arch:                 runtime.GOARCH,
              BuildTime:            BuildTime,
       }
}

func ShowVersion() {
       fmt.Println("=====================================================")
       fmt.Printf("HiPaaS scheduler version: %s\n", version.Version)
       fmt.Printf("Scheduler compatible minimum minion version: %s\n", version.MinionMinimumVersion)
       fmt.Printf("Scheduler build time: %s\n", version.BuildTime)
       fmt.Printf("Go version: %s\n", version.GoVersion)
       fmt.Printf("Git commit: %s\n", version.GitCommit)
       fmt.Printf("OS/Arch: %s/%s\n", version.Os, version.Arch)
       fmt.Println("=====================================================")
}
