package fastsetting_test
 
import (
       . "github.com/onsi/ginkgo"
       "github.com/onsi/ginkgo/reporters"
       . "github.com/onsi/gomega"
       "testing"
)
 
func TestHbs(t *testing.T) {
       RegisterFailHandler(Fail)
       junitReporter := reporters.NewJUnitReporter("fastsetting.junit.xml")
       RunSpecsWithDefaultAndCustomReporters(t, "fastsetting suite", []Reporter{junitReporter})
}
