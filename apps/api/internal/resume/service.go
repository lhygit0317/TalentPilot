package resume

import (
	"context"
	"strings"

	"github.com/talentpilot/talentpilot/apps/api/internal/audit"
	"github.com/talentpilot/talentpilot/apps/api/internal/iam"
)

const (
	jobTypeResumeImport = "resume_import"
	maxPDFBytes         = 10 * 1024 * 1024
)

type Store interface {
	List(context.Context, ListQuery) (ListResult, error)
	Get(context.Context, string, iam.ScopePredicate) (Detail, error)
	Delete(context.Context, string, iam.ScopePredicate, iam.ScopePredicate) error
	CreateImportJob(context.Context, string, bool, ImportJobInput) (string, error)
	MarkImportJobSucceeded(context.Context, string, JobStatus) error
	MarkImportJobFailed(context.Context, string, string, JobStatus) error
	GetJob(context.Context, string, string) (JobStatus, error)
	CreateImportedResume(context.Context, CreateImportedResumeInput) (string, error)
}

type Service struct {
	store  Store
	parser Parser
	audit  audit.Recorder
}

func NewService(store Store, parser Parser, recorder audit.Recorder) *Service {
	if parser == nil {
		parser = NewPDFParser()
	}
	if recorder == nil {
		recorder = audit.NopRecorder{}
	}
	return &Service{store: store, parser: parser, audit: recorder}
}

func (s *Service) List(ctx context.Context, query ListQuery) (ListResult, error) {
	return s.store.List(ctx, query)
}

func (s *Service) Get(ctx context.Context, id string, scope iam.ScopePredicate) (Detail, error) {
	return s.store.Get(ctx, id, scope)
}

func (s *Service) Delete(ctx context.Context, id string, resumeScope iam.ScopePredicate, departmentResumeScope iam.ScopePredicate) error {
	if err := s.store.Delete(ctx, id, resumeScope, departmentResumeScope); err != nil {
		return err
	}
	s.recordAudit(ctx, audit.Event{
		Type:     audit.EventResumeDeleted,
		UserID:   "",
		Resource: "Resume",
		Action:   "Delete",
		TargetID: id,
		Result:   "succeeded",
		After:    map[string]any{"resumeId": id},
	})
	return nil
}

func (s *Service) ImportOne(ctx context.Context, input ImportInput) (JobStatus, error) {
	if err := validatePDF(input.File); err != nil {
		return JobStatus{}, err
	}
	targetDepartmentID, err := resolveTargetDepartment(input.TargetDepartmentID, input.DataScope)
	if err != nil {
		return JobStatus{}, err
	}
	if !canCreateResumeInTarget(input.ResumeCreateScope, targetDepartmentID, input.Channel) ||
		!canCreateDepartmentResumeInTarget(input.DepartmentResumeCreateScope, targetDepartmentID) {
		return JobStatus{}, ErrImportScopeDenied
	}
	jobID, err := s.store.CreateImportJob(ctx, input.UserID, false, ImportJobInput{
		Channel:            input.Channel,
		TargetDepartmentID: targetDepartmentID,
		FileNames:          []string{input.File.FileName},
	})
	if err != nil {
		return JobStatus{}, err
	}

	parsed, err := s.parser.Parse(ctx, ParseInput{FileName: input.File.FileName, ContentType: input.File.ContentType, Bytes: input.File.Bytes})
	if err != nil {
		status := failedJob(jobID, []JobResult{failedResult(input.File.FileName, "RESUME_IMPORT_PARSE_FAILED")})
		if markErr := s.store.MarkImportJobFailed(ctx, jobID, "RESUME_IMPORT_PARSE_FAILED", status); markErr != nil {
			return status, markErr
		}
		s.recordAudit(ctx, audit.Event{
			Type:     audit.EventResumeImportFailed,
			UserID:   input.UserID,
			Resource: "Resume",
			Action:   "Create",
			TargetID: "",
			Result:   "failed",
			After:    map[string]any{"chan": input.Channel, "targetDepartmentId": targetDepartmentID, "jobId": jobID, "errorCode": "RESUME_IMPORT_PARSE_FAILED"},
		})
		return status, ErrParseFailed
	}

	resumeID, err := s.store.CreateImportedResume(ctx, CreateImportedResumeInput{
		UserID:             input.UserID,
		Channel:            input.Channel,
		TargetDepartmentID: targetDepartmentID,
		SourceBy:           input.UserName,
		Parsed:             parsed,
	})
	if err != nil {
		return JobStatus{}, err
	}
	status := succeededJob(jobID, []JobResult{{FileName: input.File.FileName, Status: "succeeded", ResumeID: resumeID, Name: parsed.Name}})
	if err := s.store.MarkImportJobSucceeded(ctx, jobID, status); err != nil {
		return JobStatus{}, err
	}
	s.recordAudit(ctx, audit.Event{
		Type:     audit.EventResumeImportSucceeded,
		UserID:   input.UserID,
		Resource: "Resume",
		Action:   "Create",
		TargetID: resumeID,
		Result:   "succeeded",
		After:    map[string]any{"resumeId": resumeID, "chan": input.Channel, "targetDepartmentId": targetDepartmentID, "jobId": jobID},
	})
	return status, nil
}

func (s *Service) ImportBatch(ctx context.Context, input BatchImportInput) (JobStatus, error) {
	targetDepartmentID, err := resolveTargetDepartment(input.TargetDepartmentID, input.DataScope)
	if err != nil {
		return JobStatus{}, err
	}
	if !canCreateResumeInTarget(input.ResumeCreateScope, targetDepartmentID, input.Channel) ||
		!canCreateDepartmentResumeInTarget(input.DepartmentResumeCreateScope, targetDepartmentID) {
		return JobStatus{}, ErrImportScopeDenied
	}
	fileNames := make([]string, 0, len(input.Files))
	for _, file := range input.Files {
		if err := validatePDF(file); err != nil {
			return JobStatus{}, err
		}
		fileNames = append(fileNames, file.FileName)
	}
	jobID, err := s.store.CreateImportJob(ctx, input.UserID, true, ImportJobInput{
		Channel:            input.Channel,
		TargetDepartmentID: targetDepartmentID,
		FileNames:          fileNames,
	})
	if err != nil {
		return JobStatus{}, err
	}

	results := make([]JobResult, 0, len(input.Files))
	for _, file := range input.Files {
		parsed, err := s.parser.Parse(ctx, ParseInput{FileName: file.FileName, ContentType: file.ContentType, Bytes: file.Bytes})
		if err != nil {
			results = append(results, failedResult(file.FileName, "RESUME_IMPORT_PARSE_FAILED"))
			continue
		}
		resumeID, err := s.store.CreateImportedResume(ctx, CreateImportedResumeInput{
			UserID:             input.UserID,
			Channel:            input.Channel,
			TargetDepartmentID: targetDepartmentID,
			SourceBy:           input.UserName,
			Parsed:             parsed,
		})
		if err != nil {
			return JobStatus{}, err
		}
		results = append(results, JobResult{FileName: file.FileName, Status: "succeeded", ResumeID: resumeID, Name: parsed.Name})
	}
	status := jobFromResults(jobID, results)
	if status.Summary.Failed > 0 && status.Summary.Succeeded == 0 {
		if err := s.store.MarkImportJobFailed(ctx, jobID, "RESUME_IMPORT_PARSE_FAILED", status); err != nil {
			return JobStatus{}, err
		}
		return status, nil
	}
	if err := s.store.MarkImportJobSucceeded(ctx, jobID, status); err != nil {
		return JobStatus{}, err
	}
	return status, nil
}

func (s *Service) GetJob(ctx context.Context, jobID string, userID string) (JobStatus, error) {
	return s.store.GetJob(ctx, jobID, userID)
}

func validatePDF(file ImportFile) error {
	if len(file.Bytes) == 0 {
		return ErrEmptyFile
	}
	if len(file.Bytes) > maxPDFBytes {
		return ErrFileTooLarge
	}
	nameOK := strings.HasSuffix(strings.ToLower(file.FileName), ".pdf")
	typeOK := file.ContentType == "application/pdf" || file.ContentType == "application/octet-stream"
	if !nameOK || !typeOK {
		return ErrUnsupportedFileType
	}
	return nil
}

func resolveTargetDepartment(inputTarget string, scope iam.DataScope) (string, error) {
	if inputTarget != "" && scope.AllDepartments {
		return inputTarget, nil
	}
	allowed := nonSystemDepartmentIDs(scope.Departments)
	if inputTarget == "" {
		if len(allowed) == 1 {
			return allowed[0], nil
		}
		return "", ErrImportTargetRequired
	}
	for _, departmentID := range allowed {
		if departmentID == inputTarget {
			return inputTarget, nil
		}
	}
	return "", ErrImportTargetInvalid
}

func nonSystemDepartmentIDs(departments []iam.DepartmentScope) []string {
	var ids []string
	for _, department := range departments {
		if department.ID != "" && department.ID != iam.SystemDepartmentID {
			ids = append(ids, department.ID)
		}
	}
	return ids
}

func canCreateResumeInTarget(scope iam.ScopePredicate, departmentID string, channel Channel) bool {
	return scopeAllowsImportTarget(scope, departmentID, string(channel))
}

func canCreateDepartmentResumeInTarget(scope iam.ScopePredicate, departmentID string) bool {
	return scopeAllowsImportTarget(scope, departmentID, "")
}

func scopeAllowsImportTarget(scope iam.ScopePredicate, departmentID string, channel string) bool {
	for _, branch := range scope.Branches {
		if !branch.AllDepartments && !containsString(branch.DepartmentIDs, departmentID) {
			continue
		}
		if channel != "" && len(branch.Channels) > 0 && !containsString(branch.Channels, channel) {
			continue
		}
		return true
	}
	return false
}

func succeededJob(jobID string, results []JobResult) JobStatus {
	status := jobFromResults(jobID, results)
	status.Status = "succeeded"
	return status
}

func failedJob(jobID string, results []JobResult) JobStatus {
	status := jobFromResults(jobID, results)
	status.Status = "failed"
	return status
}

func jobFromResults(jobID string, results []JobResult) JobStatus {
	status := JobStatus{ID: jobID, Type: jobTypeResumeImport, Status: "succeeded", Results: results}
	status.Summary.Total = len(results)
	for _, result := range results {
		if result.Status == "succeeded" {
			status.Summary.Succeeded++
		} else {
			status.Summary.Failed++
		}
	}
	if status.Summary.Failed > 0 && status.Summary.Succeeded == 0 {
		status.Status = "failed"
	}
	return status
}

func failedResult(fileName string, code string) JobResult {
	return JobResult{FileName: fileName, Status: "failed", ErrorCode: code}
}

func (s *Service) recordAudit(ctx context.Context, event audit.Event) {
	_ = s.audit.Record(ctx, event)
}
