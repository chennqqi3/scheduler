package fastsetting_test
 
import (
       "github.com/garyburd/redigo/redis"
       . "github.com/onsi/ginkgo"
       . "github.com/onsi/gomega"
       . "github-beta.huawei.com/hipaas/scheduler/fastsetting"
       "github-beta.huawei.com/hipaas/common/redisoperate"
       "github-beta.huawei.com/hipaas/common/fastsetting"
       "github-beta.huawei.com/hipaas/scheduler/g"
 
       "math/rand"
       "time"
       // "fmt"
)
 
var _ = Describe("Redisqueue",func(){
       var (
              redisConPool *redis.Pool
              rc                    redis.Conn
              job           *Job
              jobqueue        *JobQueue
       )
       const (
              fastsettingTestJobQueue = "harlan_test_fastsetting_job"
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
       g.ParseConfig("../cfg.example.json")
       redisConPool,_ = redisoperate.InitRedisConnPool(g.Config().Redis)
       jobqueue = new(JobQueue)
 
       BeforeEach(func(){
              rc = redisConPool.Get()
              Expect(rc).NotTo(BeNil())
              job = &Job{
                     JobID: randomstr("hcyunitTest",10),
                     JobContent: &fastsetting.FastSetting{
                            AppName: randomstr("hcyCtnrTest",10),
                            OperateType: fastsetting.Maint,
                            Containers:  []fastsetting.CtnrStatus{
                                   {
                                          ID: randomstr("hcyCtnrIDTest",10),
                                          Status: "Up",
                                   },
                            },
                     },
              }
       })
       AfterEach(func(){
              rc.Close()
       })
 
       Describe("Enqueue/Dequeue/QueuedJobCount",func(){
              JustBeforeEach(func(){
                     // Enqueue
                     err := jobqueue.Enqueue(rc,fastsettingTestJobQueue,job)
                     Expect(err).NotTo(HaveOccurred())
              })
              AfterEach(func () {
                     // Dequeue
                     result,err := jobqueue.Dequeue(rc,fastsettingTestJobQueue)
                     Expect(err).NotTo(HaveOccurred())
                     Expect(*result).To(Equal(*job))
              })
 
              It("QueuedJobCount",func () {
                     result,err := jobqueue.QueuedJobCount(rc,fastsettingTestJobQueue)
                     Expect(err).NotTo(HaveOccurred())
                     Expect(result).To(Equal(1))          
              })
       })
       Describe("Dequeue",func(){
              JustBeforeEach(func () {
                     // Enqueue
                     err := jobqueue.Enqueue(rc,fastsettingTestJobQueue,job)
                     Expect(err).NotTo(HaveOccurred())                  
              })
              AfterEach(func () {
                     jobqueue.Dequeue(rc,fastsettingTestJobQueue)
              })
              It("When Dequeue repeaty twice",func () {
                     By("Step 1,Dequeue",func () {
                            result,err := jobqueue.Dequeue(rc,fastsettingTestJobQueue)
                            Expect(err).NotTo(HaveOccurred())
                            Expect(*result).To(Equal(*job))
                     })
                     By("Step 2,Dequeue repeaty",func (){
                            result,err := jobqueue.Dequeue(rc,fastsettingTestJobQueue)
                            Expect(err).NotTo(HaveOccurred())
                            Expect(result).To(BeNil())                          
                     })
              })
              It("When queue not exist",func(){
                     result,err := jobqueue.Dequeue(rc,"notexisttestjobqueue")
                     Expect(err).NotTo(HaveOccurred())
                     Expect(result).To(BeNil())
              })
       })
 
})
