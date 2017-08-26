package sync
 
import (
       "fmt"
       "reflect"
       "strings"
       "sync"
       "time"
 
       ME "github-beta.huawei.com/hipaas/common/msgengine"
       appStorage "github-beta.huawei.com/hipaas/common/storage/app"
       "github-beta.huawei.com/hipaas/common/storage/realstate"
       "github-beta.huawei.com/hipaas/glog"
       "github-beta.huawei.com/hipaas/scheduler/g"
)
 
const (
       AppExecModeNormal  = 0
       AppExecModeUpgrade = 1
       UpdateImage        = "Image"
       UpdateEnv          = "Env"
       UpdateCPU          = "CPU"
       UpdateMem          = "Memory"
       UpdateMount        = "Mount"
       UpdateCMD          = "CMD"
)

type updateFunc func(app *appStorage.App, ctnr *realstate.Container) bool
 
var strategy = map[string]updateFunc{
       UpdateImage: func(app *appStorage.App, ctnr *realstate.Container) bool {
              if app.Image.DockerImageURL != "" && ctnr.Image != app.Image.DockerImageURL {
                     return true
              } else {
                     return false
              }
       },
       UpdateEnv: func(app *appStorage.App, ctnr *realstate.Container) bool {
              fatherMap := make(map[string]string, 0)
              for _, father := range app.Envs {
                     fatherMap[father.K] = father.V
              }
              sonMap := make(map[string]string, 0)
              for _, son := range ctnr.Envs {
                     sonMap[son.K] = son.V
              }
              return !reflect.DeepEqual(fatherMap, sonMap)
       },
	   UpdateCPU: func(app *appStorage.App, ctnr *realstate.Container) bool {
              if app.CPU == ctnr.CPU {
                     return false
              } else {
                     return true
              }
       },
       UpdateMem: func(app *appStorage.App, ctnr *realstate.Container) bool {
              if app.Memory == ctnr.Memory {
                     return false
              } else {
                     return true
              }
       },
       UpdateCMD: func(app *appStorage.App, ctnr *realstate.Container) bool {
              if app.Cmd == ctnr.Archive.Cmd {
                     return false
              } else {
                     return true
              }
       },
	   UpdateMount: func(app *appStorage.App, ctnr *realstate.Container) bool {
              mountsString, err := app.Mount.ToString()
              if err != nil {
                     return false
              }
              return mountsString != ctnr.Archive.Mount
       },
}
 
var UpdateCompareFunc = func(app *appStorage.App, ctnr *realstate.Container) bool {
       for field, exec := range strategy {
              if exec(app, ctnr) {
                     // this warning log can not be deleted.
                     glog.Warningf("app %s field %s is changed, need upgrade app.", app.Name, field)
                     return true
              }
       }
       return false
}

// Strategy Pattern of Update App
type UpdateApp interface {
       UpdateContainers(provider SchedulerProvider, app *appStorage.App) error
}
 
type AppNormal struct {
}
 
func (this *AppNormal) UpdateContainers(provider SchedulerProvider, app *appStorage.App) error {
       err := provider.UpdateContainers(app)
       if err != nil {
              glog.Error("provider.UpdateContainers fail: ", err)
       }
       return err
}
 
type AppUpgrade struct {
}

func (this *AppUpgrade) UpdateContainers(provider SchedulerProvider, app *appStorage.App) error {
       glog.Infof(`Upgrade App: %s, image: %s, instance: %d, env: %v, cpu: %d, mem: %d, cmd: %s, mount: %v`,
              app.Name, app.Image.DockerImageURL, app.Instance, app.Envs, app.CPU, app.Memory, app.Cmd, app.Mount)
       if app.Status != appStorage.AppStatusPending {
              if err := g.UpdateAppStatusByName(app.Name, appStorage.AppStatusPending, ""); err != nil {
                     return err
              }
              app.Status = appStorage.AppStatusPending
       }
 
       newCtnrUpCount := 0
       var oldCtnrs []*realstate.Container
       for _, realCtnr := range g.RealState.Containers(app.Name) {
              if UpdateCompareFunc(app, realCtnr) {
                     oldCtnrs = append(oldCtnrs, realCtnr)
                     continue
              }
			  if strings.Contains(realCtnr.Status, realstate.ContainerStatusUp) {
                     newCtnrUpCount++
              } else {
                     // abnormal containers
                     info := fmt.Sprintf("upgrade app:%s, container abnormal, id:%s, status:%s, image:%s, nodeip:%s",
                            realCtnr.AppName, realCtnr.ID, realCtnr.Status, realCtnr.Image, realCtnr.IP)
 
                     glog.Warningf(info)
 
                     ME.NewEventReporter(ME.ExecResult, ME.ExecResultData{
                            Reason:   "upgrade app, abnormal container.",
                            AppName:  realCtnr.AppName,
                            Error:    info,
                            Birthday: time.Now(),
                     })
              }
       }
	   if newCtnrUpCount >= app.Instance {
              glog.Infof(`Upgrade App "%s", Now Drop Old Instance...`, app.Name)
              ME.NewEventReporter(ME.UpgradeApp, ME.UpgradeAppData{
                     App:      *app,
                     Birthday: time.Now(),
                     Message: fmt.Sprintf("App %s upgrade --> image: %s, instance: %d, env: %v, cpu: %d, mem: %d, cmd: %s, mount: %v",
                            app.Name, app.Image.DockerImageURL, app.Instance, app.Envs, app.CPU, app.Memory, app.Cmd, app.Mount),
              })
              provider.DropContainers(app.Name, app.Instance, oldCtnrs)
              return nil
       }
	   // create new instances when app upgrade
       glog.Infof(`Upgrade App "%s", Now Create New Instance...`, app.Name)
       app.Instance -= newCtnrUpCount
       err := provider.CreateContainers(app)
       if err != nil {
              glog.Error("app update fail: ", err)
       }
       return err
}
 
type UpdateAppContext struct {
       UpdateApp
}
 
var (
       once       sync.Once
       appNormal  *AppNormal
       appUpgrade *AppUpgrade
)
 
func init() {
       once.Do(func() {
              appNormal = new(AppNormal)
              appUpgrade = new(AppUpgrade)
       })
}

func (this *UpdateAppContext) getAppExecMode(app *appStorage.App) int {
       if app.AppType != appStorage.AppTypeLRC {
              return AppExecModeNormal
       }
       oldCtnrs := g.RealState.GetOldCtnrsSliceByComparing(app, UpdateCompareFunc)
       if len(oldCtnrs) != 0 {
              return AppExecModeUpgrade
       }
       return AppExecModeNormal
}

func (this *UpdateAppContext) GetUpdateAppContext(app *appStorage.App) *UpdateAppContext {
       execMode := this.getAppExecMode(app)
       switch execMode {
       case AppExecModeNormal:
              this.UpdateApp = appNormal
       case AppExecModeUpgrade:
              this.UpdateApp = appUpgrade
       default:
              this.UpdateApp = appNormal
       }
 
       return this
}
 
func NewUpdateAppContext() *UpdateAppContext {
       return &UpdateAppContext{}
}
