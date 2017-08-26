package client
 
import (
       "bytes"
       "encoding/json"
       "errors"
       "fmt"
       "github-beta.huawei.com/hipaas/common/appevent"
       "github-beta.huawei.com/hipaas/common/fastsetting"
       "github-beta.huawei.com/hipaas/common/model"
       "github-beta.huawei.com/hipaas/common/msgengine"
       "github-beta.huawei.com/hipaas/common/storage/createstate"
       "github-beta.huawei.com/hipaas/common/storage/node"
       "github-beta.huawei.com/hipaas/common/storage/realstate"
       "github-beta.huawei.com/hipaas/glog"
       "github-beta.huawei.com/hipaas/scheduler/g"
       "github.com/tedsuo/rata"
       "io"
       "io/ioutil"
       "net/http"
       "net/url"
       "strings"
       "sync"
       "time"
)

const (
       ContentTypeHeader    = "Content-Type"
       XCfRouterErrorHeader = "X-Cf-Routererror"
       JSONContentType      = "application/json"
       HttpPrefix           = "http://"
)
 
type SchedulerConfig struct {
       Address string `json:"address"`
       Config  string `json:"config"`
       Error   string `json:"error"`
}
 
type oneClient struct {
       url        string
       httpClient *http.Client
       reqGen     *rata.RequestGenerator
}
 
func newOneClient(url string) oneClient {
       return oneClient{
              url: url,
              httpClient: &http.Client{
                     Transport: &http.Transport{
                            Proxy: nil,
                     },
              },
              reqGen: rata.NewRequestGenerator(url, Routes),
       }
}

func (c *oneClient) SetClientTimeout(timeout int) {
       c.httpClient.Timeout = time.Duration(timeout) * time.Second
}
 
func (c *oneClient) getClientUrl() string {
       return c.url
}
 
func (c *oneClient) doRequest(requestName string, params rata.Params, queryParams url.Values, request, response interface{}) error {
       req, err := c.createRequest(requestName, params, queryParams, request)
       if err != nil {
              return err
       }
       return c.do(req, response)
}

func (c *oneClient) createRequest(requestName string, params rata.Params, queryParams url.Values, request interface{}) (*http.Request, error) {
       requestJson, err := json.Marshal(request)
       if err != nil {
              return nil, err
       }
       var req *http.Request
       if request == nil {
              req, err = c.reqGen.CreateRequest(requestName, params, nil)
              requestJson = []byte("")
       } else {
              req, err = c.reqGen.CreateRequest(requestName, params, bytes.NewReader(requestJson))
       }
       if err != nil {
              return nil, err
       }
 
       req.URL.RawQuery = queryParams.Encode()
       req.ContentLength = int64(len(requestJson))
       req.Header.Set("Content-Type", "application/json")
       return req, nil
}

func (c *oneClient) do(req *http.Request, responseObject interface{}) error {
       res, err := c.httpClient.Do(req)
       if err != nil {
              return err
       }
       defer res.Body.Close()
 
       if routerError, ok := res.Header[XCfRouterErrorHeader]; ok {
              return errors.New(routerError[0])
       }
 
       return handleJSONResponse(res, responseObject)
}
 
func (c *oneClient) healthHandler() error {
       err := c.doRequest(HealthHandle, nil, nil, nil, nil)
       return err
}
 
type Client struct {
       workers []oneClient
       lock    sync.RWMutex
}

//采用长连接的方式，因为批量创建的时候申请频繁。
var LongClient = new(Client)
 
func NewClient(urls ...string) *Client {
       for _, url := range urls {
              worker := newOneClient(url)
              LongClient.workers = append(LongClient.workers, worker)
       }
       go LongClient.sortByHealth(time.Minute)
       return LongClient
}
 
func (c *Client) sortByHealth(duration time.Duration) {
       type workerResult struct {
              worker oneClient
              err    error
       }
       for {
              time.Sleep(duration)
              workers := c.getWorkers()
              if len(workers) <= 1 {
                     return
              }
			  ch := make(chan workerResult, len(workers))
              for _, worker := range workers {
                     go func(worker oneClient) {
                            err := worker.healthHandler()
                            ch <- workerResult{
                                   worker: worker,
                                   err:    err,
                            }
                     }(worker)
              }
 
              var good, bad []oneClient
              for i := 0; i < len(workers); i   {
                     select {
                     case r := <-ch:
                            if r.err != nil {
                                   bad = append(bad, r.worker)
                            } else {
                                   good = append(good, r.worker)
                            }
                     }
              }
			  workers = append(good, bad...)
 
              c.lock.Lock()
              c.workers = workers
              c.lock.Unlock()
       }
}
 
func (c *Client) getWorkers() []oneClient {
       c.lock.RLock()
       defer c.lock.RUnlock()
       workers := make([]oneClient, len(c.workers))
       copy(workers, c.workers)
       return workers
}
 
func (c *Client) doRequest(requestName string, params rata.Params, queryParams url.Values, request, response interface{}) error {
       workers := c.getWorkers()
       if len(workers) == 0 {
              return errors.New("no scheduler addresses provided")
       }
	   errInfo := "all schedulers failed to return, errors: "
       for _, worker := range workers {
              err := worker.doRequest(requestName, params, queryParams, request, response)
              if err == nil {
                     return nil
              }
              errInfo  = err.Error()   ";"
       }
       return errors.New(errInfo)
}
 
func (c *Client) HealthHandler() error {
       err := c.doRequest(HealthHandle, nil, nil, nil, nil)
       return err
}

func (c *Client) NodesHandler() (map[string][]*node.Node, error) {
       nodes := make(map[string][]*node.Node)
       err := c.doRequest(NodesHandle, nil, nil, nil, &nodes)
       return nodes, err
}
 
func (c *Client) NodeHandler(ip string) (node.Node, error) {
       var node node.Node
       err := c.doRequest(NodeHandle, rata.Params{"ip": ip}, nil, nil, &node)
       return node, err
}
 
func (c *Client) RealStateHandler() ([]*realstate.Container, error) {
       var containers []*realstate.Container
       err := c.doRequest(RealStateHandle, nil, nil, nil, &containers)
       return containers, err
}

func (c *Client) AppHandler(AppName string) ([]*realstate.Container, error) {
       var containers []*realstate.Container
       err := c.doRequest(AppHandle, rata.Params{"name": AppName}, nil, nil, &containers)
       return containers, err
}
 
func (c *Client) SetContainerStatus(fastSetting *fastsetting.FastSetting) (*createstate.SettingResponse, error) {
       var responese *createstate.SettingResponse
       err := c.doRequest(SetContainerStatus, nil, nil, fastSetting, &responese)
       return responese, err
}
 
func (c *Client) GetContainerStatus(taskID string) (*fastsetting.FastSetting, error) {
       var fastSetting *fastsetting.FastSetting
       err := c.doRequest(GetContainerStatus, rata.Params{"jobid": taskID}, nil, nil, &fastSetting)
       return fastSetting, err
}

func (c *Client) GetDockerLogs(AppName string, ctnrID string) ([]*createstate.DockerLogs, error) {
       var totalLogs []*createstate.DockerLogs
       err := c.doRequest(GetDockerLogs, nil, url.Values{"name": []string{AppName}, "id": []string{ctnrID}}, nil, &totalLogs)
       return totalLogs, err
}
 
func (c *Client) FindVmPoolResource() ([]*g.VmPoolResource, error) {
       var vmPoolResources []*g.VmPoolResource
       err := c.doRequest(FindVmPoolResource, nil, nil, nil, &vmPoolResources)
       return vmPoolResources, err
}
 
func (c *Client) GlobalTopologyHandler() (*model.GlobalLevel, error) {
       var globaltopology *model.GlobalLevel
       err := c.doRequest(GlobalTopologyHandle, nil, nil, nil, &globaltopology)
       return globaltopology, err
}

func (c *Client) GetAppEvents(appName string, size int64) (*[]appevent.EventInfo, error) {
       var events *[]appevent.EventInfo
       v := url.Values{}
       v.Add("size", fmt.Sprintf("%d", size))
       err := c.doRequest(GetAppEvents, rata.Params{"name": appName}, v, nil, &events)
       return events, err
}
 
func (c *Client) GetAppErrorEvents(appName string) (*[]appevent.EventInfo, error) {
       var events *[]appevent.EventInfo
       err := c.doRequest(GetAppErrorEvents, rata.Params{"name": appName}, nil, nil, &events)
       return events, err
}
 
func (c *Client) ListArchiveEvents() (*[]msgengine.EventArchive, error) {
       var events *[]msgengine.EventArchive
       err := c.doRequest(ListArchiveEvents, nil, nil, nil, &events)
       return events, err
}

func (c *Client) GetSchedulerConfig() (*[]SchedulerConfig, error) {
       workers := c.getWorkers()
       if len(workers) == 0 {
              return nil, errors.New("no scheduler addresses provided")
       }
 
       ch := make(chan SchedulerConfig, len(workers))
       for _, worker := range workers {
              go func(worker oneClient) {
                     var sconfig SchedulerConfig
                     var config g.GlobalConfig
 
                     // set http client timeout 1s
                     worker.SetClientTimeout(1)
                     err := worker.doRequest(GetSchedulerConfig, nil, nil, nil, &config)
                     if err != nil {
                            sconfig.Error = err.Error()
                     } else {
                            if js, err := json.Marshal(config); err != nil {
                                   sconfig.Error = err.Error()
                                   glog.Infof("parse scheduler config failed:%v", err)
								   } else {
                                   sconfig.Config = string(js)
                            }
                     }
 
                     url := worker.getClientUrl()
                     if strings.HasPrefix(url, HttpPrefix) {
                            url = strings.TrimPrefix(url, HttpPrefix)
                     }
                     ipport := strings.Split(url, ":")
                     sconfig.Address = ipport[0]
                     ch <- sconfig
              }(worker)
       }
 
       var configs []SchedulerConfig
       for i := 0; i < len(workers); i   {
              select {
              case result := <-ch:
                     configs = append(configs, result)
              }
       }
       return &configs, nil
}

func handleJSONResponse(res *http.Response, responseObject interface{}) error {
       if res.StatusCode > 299 {
              body, err := ioutil.ReadAll(res.Body)
              if err != nil {
                     return err
              }
              return errors.New(string(body))
       }
       if responseObject == nil {
              return nil
       }
       err := json.NewDecoder(res.Body).Decode(responseObject)
       if err != nil {
              return err
       }
       return nil
}

func handleNonJSONResponse(res *http.Response) error {
       if res.StatusCode > 299 {
              var errResponse []byte
              _, err := io.ReadFull(res.Body, errResponse)
              if err != nil {
                     return err
              }
              return errors.New(string(errResponse))
       }
 
       return nil
}
