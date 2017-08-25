package g
 
import (
       "fmt"
       "time"
 
       "github-beta.huawei.com/hipaas/common/crypto/aes"
       "github-beta.huawei.com/hipaas/common/storage/app"
       "github.com/astaxie/beego/orm"
       _ "github.com/go-sql-driver/mysql"
)
 
// InitDbConnPool init mysql instance.
func NewDbMgr() error {
       cfg := Config().DB
       myaes, err := aes.New("")
       if err != nil {
              return fmt.Errorf("create Crypto fail: %s", err)
       }
       pass, err := myaes.Decrypt(cfg.Password)
       if err != nil {
              return fmt.Errorf("Encrypt fail:: %s", err)
       }
       dbDsn := cfg.UserName + ":" + pass + cfg.Dsn
 
       orm.RegisterDriver("mysql", orm.DRMySQL)
       orm.RegisterDataBase("default", "mysql", dbDsn, cfg.MaxIdle, cfg.MaxOpen)
       orm.RunSyncdb("default", false, true)
       db, err := orm.GetDB()
if err != nil {
              return fmt.Errorf("get default DB fail: %s", err.Error())
       }
       db.SetConnMaxLifetime(time.Duration(int64(cfg.MaxLifetime)) * time.Second)
       return nil
}
 
// UpdateAppStatusByName update APP state in DB.
func UpdateAppStatusByName(appName string, status string, log string) error {
       return app.UpdateAppStatusByName(appName, status, log)
}
 
// UpdateHostnameStatus update APP state in DB.
func UpdateHostnameStatus(oneApp *app.App, hostname string, status string) error {
       return app.UpdateHostnameStatus(oneApp, hostname, status)
}
 
func UpdateRecoveryByName(appName string, recovery bool) error {
       return app.UpdateRecoveryByName(appName, recovery)
}

func GetAppByName(appName string) (*app.App, error) {
       return app.GetAppByName(appName)
}
 
func GetDomainUrisMapByAppName(sAppName string) (map[string]string, error) {
       return app.GetDomainUrisMapByAppName(sAppName)
}
 
func GetDesiredState() (map[string]*app.App, error) {
       return app.GetDesiredState()
}
 
func GetRequireResourceState() (map[string]*app.App, error) {
       return app.GetRequireResourceState()
}
 
func GetHealthCheckState() (map[string]*app.App, error) {
       return app.GetHealthCheckState()
}

func GetAppByStatus(status string, moreStatus ...string) (map[string]*app.App, error) {
       return app.GetAppByStatus(status, moreStatus...)
}
 
func GetStatusByAppName(appName string) (string, error) {
       return app.GetStatusByAppName(appName)
}
 
func GetShrinkedTask(appName, jobStatus string) ([]string, error) {
       return app.GetShrinkedTask(appName, jobStatus)
}
 
func GetOrphanNetworkApps() ([]*app.App, error) {
       return app.GetOrphanNetworkApps()
}
