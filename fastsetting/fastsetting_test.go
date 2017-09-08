package fastsetting_test
 
import (
       "fmt"
       "github-beta.huawei.com/hipaas/common/fastsetting"
       "github-beta.huawei.com/hipaas/common/redisoperate"
       "github-beta.huawei.com/hipaas/common/storage/realstate"
       // "github-beta.huawei.com/hipaas/common/util/taskpool"
       storageapp "github-beta.huawei.com/hipaas/common/storage/app"
       . "github-beta.huawei.com/hipaas/scheduler/fastsetting"
       "github-beta.huawei.com/hipaas/scheduler/g"
       docker "github.com/fsouza/go-dockerclient"
       . "github.com/onsi/ginkgo"
       . "github.com/onsi/gomega"
       "math/rand"
       "time"
)
 
// type FastSetting interface {
//    AddFastSettingJob(fastSetting *FS.FastSetting) (string, error)
//    GetFastSettingResult(uuid string) ([]byte, error)
//    GetAllCTNRExecMode() (map[string]*TaskStatus, error)
//    GetCTNRExecMode(containerID string) (*TaskStatus, error)
//    AddCTNRExecMode(appName string, containerID string, operateType string) error
//    DeleteCTNRExecMode(containerID string) error
//    UpdateCtnrExecMode(appName string, containerID string, operateType string) error
//    IsCTNRExecModeExist(containerId string) (bool, error)
// }
var _ = Describe("Fastsetting", func() {
       var (
              c                                           *realstate.Container
              myfastSettingMainDriver    FastSetting
              driver                                        = "redis"
              err                                     error
       )
 
       const (
              myfastSettingJobPrefix = "hipaas_fastsetting_job_"
              myfastSettingJobQueue  = "hipaas_fastsetting_job"
       )
 
       randomstr := func(prefix string,lns int) string{
              str := "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
              bytes := []byte(str)
              result := []byte{}
              r := rand.New(rand.NewSource(time.Now().UnixNano()))
              for i :=0;i < lns; i++ {
                     result = append(result,bytes[r.Intn(len(bytes))])
              }
              return prefix+string(result)
       }
 
       // prepare actions
       g.FlagInit()
       err = g.ParseConfig("../cfg.example.json")
       if err != nil {
              Fail("g.ParseConfig fail: "+err.Error())
       }
       redisPool,err := redisoperate.InitRedisConnPool(g.Config().Redis)
       if err != nil {
              Fail("redisoperate.InitRedisConnPool fail: "+err.Error())
       }
       realState, err := realstate.NewSafeRealState(driver, redisPool)
       if err != nil {
              Fail("realstate.NewSafeRealState fail: "+err.Error())
       }
       myfastSettingMainDriver = NewFastsetting(redisPool, realState)
 
       BeforeEach(func() {
              c = &realstate.Container{
                     ID:        randomstr("hcyTestFSCtnr",10),
                     IP:        "127.0.0.1",
                     Image:     "10.63.121.36:5000/official/tomcat:latest",
                     Recovery:  false,
                     AppName:   randomstr("hcyTestFSAppName",10),
                     Ports:     []*realstate.Port{&realstate.Port{PublicPort: 3128}},
                     Status:    "Up 2 minutes",
                     Hostname:  "cloud.huawei.com",
                     Region:    "Net67",
                     VMType:    "was",
                     CPU:       1,
                     Memory:    128,
                     PortInfos: []realstate.PortInfo{realstate.PortInfo {Scanner: "public", Ports: [] docker.PortBinding docker.PortBinding {{HostIP: "10.63.67.130", HostPort: "3128"}}, OriginPort: "8080"}},
                     APPTYPE: "LRP"
                     NVS: [] storageapp.Env storageapp.Env {{K: "gopath" V "/ home / go"}},
              }
              realState.UpdateContainer (c)
       })
       AfterEach (func () {
              realState.DeleteContainer (c.AppName, c)
       })
       Describe ( "AddFastSettingJob", func () {
              var fs * fastsetting.FastSetting
              var ctnrs [] * realstate.Container
              var string mosque
 
              JustBeforeEach (func () {
                     fs = {& fastsetting.FastSetting
                            AppName: c.AppName,
                            OperateType: fastsetting.Normal,
                     }
                     ctnrs = realState.Containers (c.AppName)
                     for _, ctnr: = range ctnrs {
                            fs.Containers = append (fs.Containers, fix.CtnrStatus {ID: ctnr.ID, Status: Fixation. Normal})
                     }
              })
              AfterEach (func () {
                     // delete job
                     if jid! = "" {
                            rc: = redisPool.Get ()
                            tmpKey: = fmt.Sprintf ("% s% s", myfastSettingJobPrefix, jid)
                            rc.Do ( "DEL", tmpKey)
                            rc.Do ( "RPOP" myfastSettingJobQueue)
                            rc.Close ()
 
                     }
              })
 
              It ("When OperateType is not support", func () {
                     fs.OperateType = "notsupportType"
 
                     jid, err = myfastSettingMainDriver.AddFastSettingJob (fs)
                     Expect (err) .Two (HaveOccurred ())
                     Expect (JID) .Two (Equal(""))
                     time.Sleep(time.Duration(2) * time.Second)
              })
              It("When Ctnrs is not exist",func () {
                     fs.OperateType = fastsetting.Maint
                     fs.Containers = append(fs.Containers,fastsetting.CtnrStatus{ID: "notexistTestctnrid", Status: fastsetting.Normal})
 
                     jid,err = myfastSettingMainDriver.AddFastSettingJob(fs)
                     Expect(err).To(HaveOccurred())
                     Expect(jid).To(Equal(""))
                     time.Sleep(time.Duration(2) * time.Second)
              })
              It("When add Maint",func () {
                     fs.OperateType = fastsetting.Maint
 
                     jid,err = myfastSettingMainDriver.AddFastSettingJob(fs)
                     Expect(err).NotTo(HaveOccurred())
                     Expect(jid).NotTo(Equal(""))
                     time.Sleep(time.Duration(2) * time.Second)
              })
              It("When add Restart",func () {
                     fs.OperateType = fastsetting.Restart
 
                     jid,err = myfastSettingMainDriver.AddFastSettingJob(fs)
                     Expect(err).NotTo(HaveOccurred())
                     Expect(jid).NotTo(Equal(""))
                     time.Sleep(time.Duration(2) * time.Second)
              })
              It("When add Normal",func () {
                     fs.OperateType = fastsetting.Normal
 
                     jid,err = myfastSettingMainDriver.AddFastSettingJob(fs)
                     Expect(err).NotTo(HaveOccurred())
                     Expect(jid).NotTo(Equal(""))
                     time.Sleep (time.Duration (2) * time.Second)
              })
       })
       Describe ("GetFastSettingResult", func () {
              was fs * fixing. FastSetting
              was ctnrs [] * realstate.Container
              was jid string
 
              JustBeforeEach (func () {
                     fs = & fix.FastSetting {
                            AppName: c.AppName,
                            OperateType: fixation.Normal,
                     }
                     ctnrs = realState.Containers (c.AppName)
                     for _, ctnr: = range ctnrs {
                            fs.Containers = append (fs.Containers, fix.CtnrStatus {ID: ctnr.ID, Status: Fixation. Normal})
                     }
              })
              AfterEach (func () {
                     // delete job
                     if jid! = "" {
                            rc: = redisPool.Get()
                            tmpKey := fmt.Sprintf("%s%s",myfastSettingJobPrefix,jid)
                            rc.Do("DEL",tmpKey)
                            rc.Do("RPOP",myfastSettingJobQueue)
                            rc.Close()
 
                     }                   
              })
 
              It("Testcase",func () {
                     By("Step 1,AddFastSettingJob",func () {
                            fs.OperateType = fastsetting.Maint
 
                            jid,err = myfastSettingMainDriver.AddFastSettingJob(fs)
                            Expect(err).NotTo(HaveOccurred())
                            Expect(jid).NotTo(Equal(""))
                     })
                     By("Step 2,GetFastSettingResult by not exist jid",func () {
                            _,err := myfastSettingMainDriver.GetFastSettingResult("noexistTestJid")      
                            Expect(err).To(HaveOccurred())
                     })
                     By("Step 2,GetFastSettingResult wen result is nil",func () {
                            _,err := myfastSettingMainDriver.GetFastSettingResult(jid)     
                            Expect(err).To(HaveOccurred())         
                     })
                     By("Step 3,GetFastSettingResult after result inwrite",func () {
                            rc := redisPool.Get()
                            tmpKey := fmt.Sprintf("%s%s",myfastSettingJobPrefix,jid)
                            rc.Do("HMSET",tmpKey,"result","ok")
                            rc.Close()
                            result,err := myfastSettingMainDriver.GetFastSettingResult(jid)
                            Expect(err).NotTo(HaveOccurred())
                            Expect(result).To(Equal([]byte("ok")))
                     })
              })
       })
 
       Describe("CTNRExecMode Test",func () {
              var (
                     ctnrID      string
                     otype       string
                     ctnrname    string
              )
 
              JustBeforeEach(func () {
                     // add
                     ctnrID = randomstr("hcyCTNREMtestID",10)
                     otype = fastsetting.Normal
                     ctnrname = randomstr("hcyCTNREMtestNAME",10)
 
                     err := myfastSettingMainDriver.AddCTNRExecMode(ctnrname,ctnrID,otype)
                     Expect(err).NotTo(HaveOccurred())
              })
              AfterEach(func () {
                     // delete
                     myfastSettingMainDriver.DeleteCTNRExecMode(ctnrID)
              })
 
              It("GetCTNRExecMode when ctnrid not exist",func () {
                     result,err := myfastSettingMainDriver.GetCTNRExecMode("notexistTestctnrid")
                     Expect(err).NotTo(HaveOccurred())
                     Expect(result).To(BeNil())
              })
              It("GetCTNRExecMode when Normal",func () {
                     result,err := myfastSettingMainDriver.GetCTNRExecMode(ctnrID)
                     Expect(err).NotTo(HaveOccurred())
                     Expect(result.ID).To(Equal(ctnrID))
                     Expect(result.Appname).To(Equal(ctnrname))
                     Expect(result.OperType).To(Equal(otype))
              })
              It("GetAllCTNRExecMode",func () {
                     result,err := myfastSettingMainDriver.GetAllCTNRExecMode()
                     Expect(err).NotTo(HaveOccurred())
                     Expect(result[ctnrID].ID).To(Equal(ctnrID))
                     Expect(result[ctnrID].OperType).To(Equal(otype))
              })
              It("UpdateCtnrExecMode",func () {
                     newoptype := fastsetting.Restart
                     err := myfastSettingMainDriver.UpdateCtnrExecMode(ctnrname,ctnrID,newoptype)
                     Expect(err).NotTo(HaveOccurred())
 
                     result,err := myfastSettingMainDriver.GetCTNRExecMode(ctnrID)
                     Expect(err).NotTo(HaveOccurred())
                     Expect(result.ID).To(Equal(ctnrID))
                     Expect(result.Appname).To(Equal(ctnrname))
                     Expect(result.OperType).To(Equal(newoptype))       
              })
              It("IsCTNRExecModeExist",func () {
                     By("Step 1,not exist",func () {
                            result,err := myfastSettingMainDriver.IsCTNRExecModeExist("notexistTestctnrid")
                            Expect(err).NotTo(HaveOccurred())
                            Expect(result).To(Equal(false))
                     })
                     By("Step 2,Normal",func () {
                            result,err := myfastSettingMainDriver.IsCTNRExecModeExist(ctnrID)
                            Expect(err).NotTo(HaveOccurred())
                            Expect(result).To(Equal(true))
                     })
              })
       })
})
