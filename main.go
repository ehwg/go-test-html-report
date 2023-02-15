package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"math"
	"os"
	"strings"
	"time"

	"github.com/SAP/jenkins-library/pkg/log"
	"github.com/ehwg/goTestHtmlReport/assets"
	"github.com/spf13/cobra"
)

type GoTestJsonRowData struct {
	Time    time.Time
	Action  string
	Package string
	Test    string
	Output  string
	Elapsed float64
}

type ProcessedTestdata struct {
	TotalTestTime     string
	TestDate          string
	FailedTests       int
	PassedTests       int
	TestSummary       []TestOverview
	PackageDetailsMap map[string]PackageDetails
	testDetailsOutput map[string][]string
}

type PackageDetails struct {
	Name          string
	ElapsedTime   float64
	TimeSymbol    string
	Status        string
	Coverage      string
	NumberOfTests int
}

type TestDetails struct {
	PackageName   string
	RootTest      string
	ParentTest    string
	Name          string
	ElapsedTime   float64
	TimeSymbol    string
	Status        string
	NumberOfTests int
}

type TestOverview struct {
	TestSuite TestDetails
	TestCases []TestDetails
}

func main() {
	rootCmd := initCommand()
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var coverageFileName string
var coverageReportFileName string
var outputDirectory string

func initCommand() *cobra.Command {
	var rootCmd = &cobra.Command{
		Use:   "goTestHtmlReport",
		Long:  "goTestHtmlReport generates a html report of go-test logs",
		Short: "goTestHtmlReport generates a html report of go-test logs",
		RunE: func(cmd *cobra.Command, args []string) (e error) {
			testData := make([]GoTestJsonRowData, 0)

			file, _ := cmd.Flags().GetString("file")
			if file != "" {
				fileLogData, err := ReadLogsFromFile(file)
				if err != nil {
					log.Entry().Info("error reading logs from a file")
					return err
				}

				testData = *fileLogData
			} else {
				stdInLogData, err := ReadLogsFromStdIn()
				if err != nil {
					log.Entry().Info("error reading logs from standard input ")
					return err
				}

				testData = *stdInLogData
			}

			processedTestdata, err := ProcessTestData(testData)
			if err != nil {
				log.Entry().Info("error processing test logs")
				return err
			}

			err = GenerateHTMLReport(processedTestdata.TotalTestTime,
				processedTestdata.TestDate,
				processedTestdata.FailedTests,
				processedTestdata.PassedTests,
				processedTestdata.TestSummary,
				processedTestdata.PackageDetailsMap,
				processedTestdata.testDetailsOutput,
			)
			if err != nil {
				log.Entry().Info("error generating report html")
				return err
			}

			log.Entry().Info("Test report generated successfully")
			return nil
		},
	}
	rootCmd.PersistentFlags().StringVarP(
		&coverageFileName,
		"file",
		"f",
		"",
		"set the file of the go test json logs",
	)
	rootCmd.Flags().StringVarP(
		&outputDirectory,
		"output",
		"o",
		"",
		"set the output directory of the html report",
	)
	rootCmd.Flags().StringVarP(
		&coverageReportFileName,
		"reportFile",
		"c",
		"",
		"set the file for the coverage report",
	)
	return rootCmd
}

func ReadLogsFromFile(fileName string) (*[]GoTestJsonRowData, error) {

	file, err := os.Open("C:\\Users\\d022276\\GO\\src\\go-test-html-report\\sample\\gocoverageTest.json")
	if err != nil {
		log.Entry().Info("error opening file")
		return nil, err
	}
	defer func() {
		err := file.Close()
		if err != nil {
			log.Entry().Info("error closing file")
		}
	}()

	// file scanner
	scanner := bufio.NewScanner(file)
	rowData := make([]GoTestJsonRowData, 0)
	for scanner.Scan() {
		row := GoTestJsonRowData{}
		err = json.Unmarshal([]byte(scanner.Text()), &row)
		if err != nil {
			log.Entry().Info("error unmarshalling go test logs")
			return nil, err
		}
		rowData = append(rowData, row)
	}

	if err = scanner.Err(); err != nil {
		log.Entry().Info("error scanning file")
		return nil, err
	}

	return &rowData, nil
}

func ReadLogsFromStdIn() (*[]GoTestJsonRowData, error) {
	// stdin scanner
	scanner := bufio.NewScanner(os.Stdin)
	rowData := make([]GoTestJsonRowData, 0)
	for scanner.Scan() {
		row := GoTestJsonRowData{}
		// unmarshall each line into GoTestJsonRowData
		err := json.Unmarshal([]byte(scanner.Text()), &row)
		if err != nil {
			log.Entry().Info("error unmarshalling the test logs")
			return nil, err
		}
		rowData = append(rowData, row)
	}
	if err := scanner.Err(); err != nil {
		log.Entry().Info("error with stdin scanner")
		return nil, err
	}

	return &rowData, nil
}

func ProcessTestData(rowData []GoTestJsonRowData) (*ProcessedTestdata, error) {
	packageDetailsMap := map[string]PackageDetails{}
	testDetailsOutput := map[string][]string{}
	for _, r := range rowData {
		if r.Test == "" {
			if r.Action == "fail" || r.Action == "pass" || r.Action == "skip" {
				elapsedTime, timeSymbol := formatTimeDisplay(r.Elapsed)
				packageDetailsMap[r.Package] = PackageDetails{
					Name:        r.Package,
					ElapsedTime: elapsedTime,
					TimeSymbol:  timeSymbol,
					Status:      r.Action,
					Coverage:    packageDetailsMap[r.Package].Coverage,
				}
			}
			if r.Action == "output" {
				coverage := "-"
				if strings.Contains(r.Output, "coverage") && strings.Contains(r.Output, "%") {
					coverage = r.Output[strings.Index(r.Output, ":")+1 : strings.Index(r.Output, "%")+1]
				}
				elapsedTime, timeSymbol := formatTimeDisplay(packageDetailsMap[r.Package].ElapsedTime)

				packageDetails := packageDetailsMap[r.Package]
				if elapsedTime != 0 {
					packageDetails.ElapsedTime = elapsedTime
				}
				if timeSymbol != "" {
					packageDetails.TimeSymbol = timeSymbol
				}
				if coverage != "-" {
					packageDetails.Coverage = coverage
				}

				packageDetailsMap[r.Package] = packageDetails
			}
		} else {
			if r.Output != "" && !strings.Contains(r.Output, "===") && !strings.Contains(r.Output, "---") {
				testDetailsOutputSingle := testDetailsOutput[r.Test]
				testDetailsOutputSingle = append(
					testDetailsOutputSingle,
					r.Output)
				testDetailsOutput[r.Test] = testDetailsOutputSingle
			}
		}
	}

	testSuiteSlice := make([]TestDetails, 0)
	testCasesSlice := make([]TestDetails, 0)
	passedTests := 0
	failedTests := 0
	for _, r := range rowData {
		if r.Test != "" {
			testNameSlice := strings.Split(r.Test, "/")

			// if testNameSlice is not equal 1 then we assume we have a test case information. Record test case info

			if len(testNameSlice) != 1 {
				if r.Action == "fail" || r.Action == "pass" {
					elapsedTime, timeSymbol := formatTimeDisplay(r.Elapsed)
					testCasesSlice = append(
						testCasesSlice,
						TestDetails{
							PackageName: r.Package,
							RootTest:    testNameSlice[0],
							ParentTest:  testNameSlice[len(testNameSlice)-2],
							Name:        r.Test,
							ElapsedTime: elapsedTime,
							TimeSymbol:  timeSymbol,
							Status:      r.Action,
						},
					)
				}
				switch r.Action {
				case "fail":
					failedTests = failedTests + 1
				case "pass":
					passedTests = passedTests + 1
				}
				continue
			}

			if r.Action == "fail" || r.Action == "pass" {
				elapsedTime, timeSymbol := formatTimeDisplay(r.Elapsed)
				testSuiteSlice = append(
					testSuiteSlice,
					TestDetails{
						PackageName: r.Package,
						RootTest:    testNameSlice[0],
						ParentTest:  testNameSlice[len(testNameSlice)-1],
						Name:        r.Test,
						ElapsedTime: elapsedTime,
						TimeSymbol:  timeSymbol,
						Status:      r.Action,
					})
			}
			switch r.Action {
			case "fail":
				failedTests = failedTests + 1
			case "pass":
				passedTests = passedTests + 1
			}
		}
	}

	testSummary := make([]TestOverview, 0)
	for _, t := range testSuiteSlice {
		testCases := make([]TestDetails, 0)
		for _, t2 := range testCasesSlice {
			if strings.Contains(t2.Name, t.Name) && t2.PackageName == t.PackageName {
				testCases = append(testCases, t2)
			}
		}
		testSummary = append(
			testSummary,
			TestOverview{
				TestSuite: t,
				TestCases: testCases,
			},
		)
	}

	totalTestTime := ""
	if rowData[len(rowData)-1].Time.Sub(rowData[0].Time).Seconds() < 60 {
		totalTestTime = fmt.Sprintf("%f s", rowData[len(rowData)-1].Time.Sub(rowData[0].Time).Seconds())
	} else {
		min := int(math.Trunc(rowData[len(rowData)-1].Time.Sub(rowData[0].Time).Seconds() / 60))
		seconds := int(math.Trunc((rowData[len(rowData)-1].Time.Sub(rowData[0].Time).Minutes() - float64(min)) * 60))
		totalTestTime = fmt.Sprintf("%dm:%ds", min, seconds)
	}
	testDate := rowData[0].Time.Format(time.RFC850)

	return &ProcessedTestdata{
		TotalTestTime:     totalTestTime,
		TestDate:          testDate,
		FailedTests:       failedTests,
		PassedTests:       passedTests,
		TestSummary:       testSummary,
		PackageDetailsMap: packageDetailsMap,
		testDetailsOutput: testDetailsOutput,
	}, nil
}

func GenerateHTMLReport(totalTestTime, testDate string, failedTests, passedTests int, testSummary []TestOverview, packageDetailsMap map[string]PackageDetails, testDetailsOutput map[string][]string) error {

	testCasesEl, _ := generateTestCaseHTMLElements(testSummary, testDetailsOutput)
	testSuitesEl, _ := generateTestSuiteHTMLElements(testSummary, *testCasesEl)
	packagesEl, _ := generatePackageDetailsHTMLElements(*testSuitesEl, packageDetailsMap)

	reportTemplate := template.New("reportTemplate.html")
	reportTemplateData, err := assets.Asset("reportTemplate.html")
	if err != nil {
		log.Entry().Info("error retrieving reportTemplate.html")
		return err
	}

	report, err := reportTemplate.Parse(string(reportTemplateData))
	if err != nil {
		log.Entry().Info("error parsing reportTemplate.html")
		return err
	}

	var processedTemplate bytes.Buffer
	type templateData struct {
		HTMLElements  []template.HTML
		FailedTests   int
		PassedTests   int
		TotalTestTime string
		TestDate      string
	}

	err = report.Execute(&processedTemplate,
		&templateData{
			HTMLElements:  []template.HTML{template.HTML(packagesEl)},
			FailedTests:   failedTests,
			PassedTests:   passedTests,
			TotalTestTime: totalTestTime,
			TestDate:      testDate,
		},
	)
	if err != nil {
		log.Entry().Info("error applying reportTemplate.html")
		return err
	}
	var path = ""
	if outputDirectory == "" {
		path = "./"
	} else {
		path = fmt.Sprintf("%s/", outputDirectory)
	}
	if coverageReportFileName == "" {
		path = path + "testCoverageReport.html"
	} else {
		path = path + coverageReportFileName
	}
	err = os.WriteFile(path, processedTemplate.Bytes(), 0644)
	if err != nil {
		log.Entry().Info("error writing report.html file")
		return err
	}
	return nil
}

func generateTestCaseHTMLElements(testsLogOverview []TestOverview, testDetailsOutput map[string][]string) (*map[string][]string, error) {
	testCasesCardsMap := make(map[string][]string)
	testCaseCard := template.HTML("")

	for _, testSuite := range testsLogOverview {
		for _, testCaseDetails := range testSuite.TestCases {
			if testCaseDetails.ParentTest == testCaseDetails.RootTest {
				testCaseCard = `
										<div>{{.testName}}</div>
										<div>{{.elapsedTime}}{{.timeSymbol}}</div>
									`
				testCaseTemplate, err := template.New("testCase").Parse(string(testCaseCard))
				if err != nil {
					log.Entry().Info("error parsing test case template")
					return nil, err
				}

				var processedTestCaseTemplate bytes.Buffer
				err = testCaseTemplate.Execute(&processedTestCaseTemplate, map[string]string{
					"testName":    testCaseDetails.Name,
					"elapsedTime": fmt.Sprintf("%f", testCaseDetails.ElapsedTime),
					"timeSymbol":  fmt.Sprintf("%s", testCaseDetails.TimeSymbol),
				})

				if err != nil {
					log.Entry().Info("error applying test case template")
					return nil, err
				}

				getSubTests(testCaseDetails.Name, testSuite.TestCases, testDetailsOutput)

				switch testCaseDetails.Status {
				case "pass":
					testCaseCard = template.HTML(
						fmt.Sprintf(`
												<div class="testCardLayout successBackgroundColor">
												%s
												</div>
											`,
							template.HTML(processedTestCaseTemplate.Bytes()),
						),
					)

				case "fail":
					getTestLogDetails(testDetailsOutput[testCaseDetails.Name])
					testCaseCard = template.HTML(
						fmt.Sprintf(`
												<div class="testCardLayout failBackgroundColor ">
												%s
												</div>
												`,
							template.HTML(processedTestCaseTemplate.Bytes()),
						),
					)
				}
				testCasesCardsMap[testSuite.TestSuite.Name+"-"+testSuite.TestSuite.PackageName] = append(testCasesCardsMap[testSuite.TestSuite.Name+"-"+testSuite.TestSuite.PackageName], string(testCaseCard))
			}
		}
	}

	return &testCasesCardsMap, nil
}

func generateTestSuiteHTMLElements(testLogOverview []TestOverview, testCaseHTMLCards map[string][]string) (*map[string][]string, error) {
	testSuiteCollapsibleCardsMap := make(map[string][]string)
	collapsible := template.HTML("")
	collapsibleHeading := template.HTML("")
	collapsibleHeadingTemplate := ""
	collapsibleContent := template.HTML("")

	for _, testSuite := range testLogOverview {
		collapsibleHeadingTemplate = `		
										<div>{{.testName}}</div>
										<div>{{.elapsedTime}}{{.timeSymbol}}</div>
									`
		testCaseTemplate, err := template.New("testSuite").Parse(collapsibleHeadingTemplate)
		if err != nil {
			log.Entry().Info("error parsing test case template")
			return nil, err
		}

		var processedTestCaseTemplate bytes.Buffer
		err = testCaseTemplate.Execute(&processedTestCaseTemplate, map[string]string{
			"testName":    testSuite.TestSuite.Name,
			"elapsedTime": fmt.Sprintf("%f", testSuite.TestSuite.ElapsedTime),
			"timeSymbol":  fmt.Sprintf("%s", testSuite.TestSuite.TimeSymbol),
		})
		if err != nil {
			log.Entry().Info("error applying test case template")
			return nil, err
		}

		switch testSuite.TestSuite.Status {
		case "pass":
			collapsibleHeading = template.HTML(
				fmt.Sprintf(`
											<div class="testCardLayout successBackgroundColor collapsibleHeading">
											%s
											</div>
										`,
					template.HTML(processedTestCaseTemplate.Bytes()),
				),
			)

		case "fail":
			collapsibleHeading = template.HTML(
				fmt.Sprintf(`
											<div class="testCardLayout failBackgroundColor collapsibleHeading">
											%s
											</div>
										`,
					template.HTML(processedTestCaseTemplate.Bytes()),
				),
			)
		}

		collapsibleContent = template.HTML(
			fmt.Sprintf(`
									<div class="collapsibleHeadingContent">
										%s
									</div>
							`,
				strings.Join(testCaseHTMLCards[testSuite.TestSuite.Name+"-"+testSuite.TestSuite.PackageName], "\n"),
			),
		)

		collapsible = template.HTML(
			fmt.Sprintf(`
						<div type="button" class="collapsible">
							%s
							%s
						</div>
							`,
				string(collapsibleHeading),
				string(collapsibleContent),
			),
		)

		testSuiteCollapsibleCardsMap[testSuite.TestSuite.PackageName] = append(testSuiteCollapsibleCardsMap[testSuite.TestSuite.PackageName], string(collapsible))
	}

	return &testSuiteCollapsibleCardsMap, nil
}

func generatePackageDetailsHTMLElements(testSuiteOverview map[string][]string, packageDetailsMap map[string]PackageDetails) (string, error) {
	collapsibleHeading := template.HTML("")
	collapsible := template.HTML("")
	collapsibleHeadingTemplate := ""
	collapsibleContent := template.HTML("")
	elem := make([]string, 0)
	for _, v := range packageDetailsMap {
		collapsibleHeadingTemplate = `
											<div>{{.packageName}}</div>
											<div>{{.coverage}}</div>
											<div>{{.elapsedTime}}{{.timeSymbol}}</div>
											`
		packageInfoTemplate, err := template.New("packageInfoTemplate").Parse(string(collapsibleHeadingTemplate))
		if err != nil {
			log.Entry().Info("error parsing package info template")
			os.Exit(1)
		}
		var processedPackageTemplate bytes.Buffer
		err = packageInfoTemplate.Execute(&processedPackageTemplate, map[string]string{
			"packageName": v.Name,
			"elapsedTime": fmt.Sprintf("%f", v.ElapsedTime),
			"timeSymbol":  fmt.Sprintf("%s", v.TimeSymbol),
			"coverage":    v.Coverage,
		})
		if err != nil {
			log.Entry().Info("error applying package info template")
			os.Exit(1)
		}
		switch v.Status {
		case "pass":
			collapsibleHeading = template.HTML(
				fmt.Sprintf(
					`
							<div class="collapsibleHeading packageCardLayout successBackgroundColor ">
								%s
							</div>
						`,
					template.HTML(processedPackageTemplate.Bytes()),
				),
			)
		case "fail":
			collapsibleHeading = template.HTML(
				fmt.Sprintf(
					`
							<div class="collapsibleHeading packageCardLayout failBackgroundColor ">
								%s
							</div>
						`,
					template.HTML(processedPackageTemplate.Bytes()),
				),
			)
		default:
			collapsibleHeading = template.HTML(
				fmt.Sprintf(
					`
							<div class="collapsibleHeading packageCardLayout skipBackgroundColor">
								%s
							</div>
						`,
					template.HTML(processedPackageTemplate.Bytes()),
				),
			)
		}
		collapsibleContent = template.HTML(
			fmt.Sprintf(`
									<div class="collapsibleHeadingContent">
										%s
									</div>
							`,
				strings.Join(testSuiteOverview[v.Name], "\n"),
			),
		)
		collapsible = template.HTML(
			fmt.Sprintf(`
						<div type="button" class="collapsible">
							%s
							%s
						</div>
							`,
				string(collapsibleHeading),
				string(collapsibleContent),
			),
		)
		elem = append(elem, string(collapsible))
	}
	return strings.Join(elem, "\n"), nil
}

func formatTimeDisplay(secs float64) (float64, string) {
	if secs > 1 {
		return secs, "s"
	}
	parsed, err := time.ParseDuration(fmt.Sprintf("%vs", secs))
	if err != nil {
		return 0, "ms"
	}
	return float64(parsed.Milliseconds()), "ms"
}

func getSubTests(parentTest string, testCases []TestDetails, testDetailsOutput map[string][]string) string {

	for _, testCaseDetails := range testCases {
		if testCaseDetails.ParentTest != testCaseDetails.RootTest && strings.Contains(testCaseDetails.Name, parentTest) {
			log.Entry().Info(testCaseDetails)
			getTestLogDetails(testDetailsOutput[testCaseDetails.Name])
		}
	}
	return ""
}

func getTestLogDetails(testDetailsOutput []string) string {

	if testDetailsOutput != nil {
		log.Entry().Info(testDetailsOutput)
		return "details"
	}
	return ""
}
