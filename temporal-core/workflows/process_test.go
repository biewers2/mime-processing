package workflows

import (
	"context"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
	"testing"
)

//
// Activity stubs - used to mock Rust-based activities //
//
// This is necessary because the Temporal test framework needs to use the activity function to determine
// the types of the interface to mock.
//

func CreateWorkspace(_ context.Context, _ CreateWorkspaceInput) (Workspace, error) {
	return Workspace{}, nil
}

func Download(_ context.Context, _ DownloadInput) error {
	return nil
}

func ProcessRustyFile(_ context.Context, _ ProcessRustyFileInput) (ProcessRustyFileOutput, error) {
	return ProcessRustyFileOutput{}, nil
}

func Zip(_ context.Context, _ ZipInput) (ZipOutput, error) {
	return ZipOutput{}, nil
}

func Upload(_ context.Context, _ UploadInput) error {
	return nil
}

func RemoveWorkspace(_ context.Context, _ RemoveWorkspaceInput) error {
	return nil
}

//
// Unit tests
//

type UnitTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	env *testsuite.TestWorkflowEnvironment
}

func (s *UnitTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	s.env.RegisterActivity(CreateWorkspace)
	s.env.RegisterActivity(Download)
	s.env.RegisterActivity(ProcessRustyFile)
	s.env.RegisterActivity(Zip)
	s.env.RegisterActivity(Upload)
	s.env.RegisterActivity(RemoveWorkspace)
}

func (s *UnitTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func TestUnitTestSuite(t *testing.T) {
	suite.Run(t, new(UnitTestSuite))
}

func (s *UnitTestSuite) Test_Process_RecurseFalse() {
	input := processInput()
	input.Recurse = false
	workspace := Workspace{
		RootPath:        "/tmp/mock-root-path",
		Directory:       "/tmp/mock-workspace-dir",
		StickyTaskQueue: "mock-sticky-task-queue",
	}
	downloadInput := DownloadInput{
		S3Uri: input.InputS3Uri,
		Path:  workspace.RootPath,
	}
	prfInput := ProcessRustyFileInput{
		Path:      workspace.RootPath,
		Directory: workspace.Directory,
		Mimetype:  input.MimeType,
		Types:     input.Types,
	}
	prfOutput := ProcessRustyFileOutput{
		ProcessedFiles: []FileInfo{{Path: workspace.Directory + "/mock-file-1"}},
	}
	zipInput := ZipInput{Directory: workspace.Directory}
	zipOutput := ZipOutput{Path: "/tmp/mock-zipping-dir/mock-archive.zip"}
	uploadInput := UploadInput{
		Path:  zipOutput.Path,
		S3Uri: input.OutputS3Uri,
	}
	removeWorkspaceInput := RemoveWorkspaceInput{
		Paths: []string{
			workspace.RootPath,
			workspace.Directory,
			zipOutput.Path,
		},
	}

	s.env.OnActivity(CreateWorkspace, mock.Anything, CreateWorkspaceInput{}).
		Return(workspace, nil).
		Once()
	s.env.OnActivity(Download, mock.Anything, downloadInput).
		Return(nil).
		Once()
	s.env.OnActivity(ProcessRustyFile, mock.Anything, prfInput).
		Return(prfOutput, nil).
		Once()
	s.env.OnActivity(Zip, mock.Anything, zipInput).
		Return(zipOutput, nil).
		Once()
	s.env.OnActivity(Upload, mock.Anything, uploadInput).
		Return(nil).
		Once()
	s.env.OnActivity(RemoveWorkspace, mock.Anything, removeWorkspaceInput).
		Return(nil).
		Once()

	s.env.ExecuteWorkflow(Process, input)

	s.True(s.env.IsWorkflowCompleted())
	s.Nil(s.env.GetWorkflowError())
}

func (s *UnitTestSuite) Test_Process_RecurseTrue() {
	input := processInput()
	input.Recurse = true
	workspace := Workspace{
		RootPath:        "/tmp/mock-root-path",
		Directory:       "/tmp/mock-workspace-dir/",
		StickyTaskQueue: "mock-sticky-task-queue",
	}
	downloadInput := DownloadInput{
		S3Uri: input.InputS3Uri,
		Path:  workspace.RootPath,
	}
	firstPrfInput := ProcessRustyFileInput{
		Path:      workspace.RootPath,
		Directory: workspace.Directory,
		Mimetype:  input.MimeType,
		Types:     input.Types,
	}
	firstPrfOutput := ProcessRustyFileOutput{
		EmbeddedFiles: []FileInfo{
			{
				Path:     workspace.Directory + "mock-file-2",
				MimeType: "mock/file",
			},
		},
	}
	secondPrfInput := ProcessRustyFileInput{
		Path:      firstPrfOutput.EmbeddedFiles[0].Path,
		Directory: workspace.Directory,
		Mimetype:  firstPrfOutput.EmbeddedFiles[0].MimeType,
		Types:     input.Types,
	}
	secondPrfOutput := ProcessRustyFileOutput{}
	zipInput := ZipInput{Directory: workspace.Directory}
	zipOutput := ZipOutput{Path: "/tmp/mock-zipping-dir/mock-archive.zip"}
	uploadInput := UploadInput{
		Path:  zipOutput.Path,
		S3Uri: input.OutputS3Uri,
	}
	removeWorkspaceInput := RemoveWorkspaceInput{
		Paths: []string{
			workspace.RootPath,
			workspace.Directory,
			zipOutput.Path,
		},
	}

	s.env.OnActivity(CreateWorkspace, mock.Anything, CreateWorkspaceInput{}).
		Return(workspace, nil).
		Once()
	s.env.OnActivity(Download, mock.Anything, downloadInput).
		Return(nil).
		Once()
	s.env.OnActivity(ProcessRustyFile, mock.Anything, firstPrfInput).
		Return(firstPrfOutput, nil).
		Once()
	s.env.OnActivity(ProcessRustyFile, mock.Anything, secondPrfInput).
		Return(secondPrfOutput, nil).
		Once()
	s.env.OnActivity(Zip, mock.Anything, zipInput).
		Return(zipOutput, nil).
		Once()
	s.env.OnActivity(Upload, mock.Anything, uploadInput).
		Return(nil).
		Once()
	s.env.OnActivity(RemoveWorkspace, mock.Anything, removeWorkspaceInput).
		Return(nil).
		Once()

	s.env.ExecuteWorkflow(Process, input)

	s.True(s.env.IsWorkflowCompleted())
	s.Nil(s.env.GetWorkflowError())
}

func processInput() ProcessInput {
	return ProcessInput{
		InputS3Uri:  "s3://mock-bucket/mock-input",
		OutputS3Uri: "s3://mock-bucket/mock-output",
		MimeType:    "application/octet-stream",
		Types:       []string{"Text"},
		Recurse:     false,
	}
}
