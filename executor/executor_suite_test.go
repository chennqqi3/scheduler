package executor_test
 
import (
       . "github.com/onsi/ginkgo"
       "github.com/onsi/ginkgo/reporters"
       . "github.com/onsi/gomega"
       "testing"
)
 
func TestExecutor(t *testing.T) {
       RegisterFailHandler(Fail)
       junitReporter := reporters.NewJUnitReporter("executor.junit.xml")
       RunSpecsWithDefaultAndCustomReporters(t, "executor suite", []Reporter{junitReporter})
       //RunSpecs(t, "Executor Suite")
}
