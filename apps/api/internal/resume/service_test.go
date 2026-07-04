package resume_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/talentpilot/talentpilot/apps/api/internal/audit"
	"github.com/talentpilot/talentpilot/apps/api/internal/iam"
	"github.com/talentpilot/talentpilot/apps/api/internal/resume"
)

func TestServiceSingleImportInfersOnlyConcreteDepartment(t *testing.T) {
	database := newResumeMigratedSQLiteGormDB(t)
	seedResumeFixture(t, database)
	parser := &fakeParser{results: []resume.ParsedResume{parsedResume("赵六")}}
	service := resume.NewService(resume.NewSQLStore(database), parser, audit.NopRecorder{})

	status, err := service.ImportOne(context.Background(), resume.ImportInput{
		UserID:    "user_owner",
		Channel:   resume.ChannelSocial,
		DataScope: iam.DataScope{Departments: []iam.DepartmentScope{{ID: "dept_a", Name: "算力训练平台部"}}},
		File:      pdfFile("zhaoliu.pdf"),
	})
	if err != nil {
		t.Fatalf("import one: %v", err)
	}

	if parser.calls != 1 || status.Status != "succeeded" {
		t.Fatalf("expected successful inferred import, calls=%d status=%#v", parser.calls, status)
	}
	assertResumeCount(t, database, "department_resumes", "department_id = 'dept_a' AND resume_id = '"+status.Results[0].ResumeID+"'", 1)
}

func TestServiceSingleImportRequiresTargetForMultipleDepartments(t *testing.T) {
	parser := &fakeParser{}
	service := resume.NewService(resume.NewSQLStore(newResumeMigratedSQLiteGormDB(t)), parser, audit.NopRecorder{})

	_, err := service.ImportOne(context.Background(), resume.ImportInput{
		UserID:  "user_owner",
		Channel: resume.ChannelSocial,
		DataScope: iam.DataScope{Departments: []iam.DepartmentScope{
			{ID: "dept_a", Name: "算力训练平台部"},
			{ID: "dept_b", Name: "智算调度部"},
		}},
		File: pdfFile("multi.pdf"),
	})
	if !errors.Is(err, resume.ErrImportTargetRequired) {
		t.Fatalf("expected target required, got %v", err)
	}
	if parser.calls != 0 {
		t.Fatalf("expected target validation before parser, calls=%d", parser.calls)
	}
}

func TestServiceSingleImportRejectsInvalidFileBeforeParser(t *testing.T) {
	parser := &fakeParser{}
	service := resume.NewService(resume.NewSQLStore(newResumeMigratedSQLiteGormDB(t)), parser, audit.NopRecorder{})

	_, err := service.ImportOne(context.Background(), resume.ImportInput{
		UserID:    "user_owner",
		Channel:   resume.ChannelSocial,
		DataScope: iam.DataScope{Departments: []iam.DepartmentScope{{ID: "dept_a", Name: "算力训练平台部"}}},
		File:      resume.ImportFile{FileName: "resume.txt", ContentType: "text/plain", Bytes: []byte("not a pdf")},
	})
	if !errors.Is(err, resume.ErrUnsupportedFileType) {
		t.Fatalf("expected unsupported file type, got %v", err)
	}
	if parser.calls != 0 {
		t.Fatalf("expected file validation before parser, calls=%d", parser.calls)
	}
}

func TestServiceSingleImportCreatesResumeAndDepartmentResumeInTransaction(t *testing.T) {
	database := newResumeMigratedSQLiteGormDB(t)
	seedResumeFixture(t, database)
	parser := &fakeParser{results: []resume.ParsedResume{parsedResume("钱七")}}
	service := resume.NewService(resume.NewSQLStore(database), parser, audit.NopRecorder{})

	status, err := service.ImportOne(context.Background(), resume.ImportInput{
		UserID:             "user_owner",
		Channel:            resume.ChannelCampus,
		TargetDepartmentID: "dept_b",
		DataScope:          iam.DataScope{AllDepartments: true},
		File:               pdfFile("qianqi.pdf"),
	})
	if err != nil {
		t.Fatalf("import one: %v", err)
	}

	if status.Summary.Succeeded != 1 || status.Results[0].Name != "钱七" {
		t.Fatalf("expected successful job result, got %#v", status)
	}
	assertResumeCount(t, database, "resumes", "id = '"+status.Results[0].ResumeID+"' AND chan = 'campus'", 1)
	assertResumeCount(t, database, "department_resumes", "department_id = 'dept_b' AND resume_id = '"+status.Results[0].ResumeID+"'", 1)
}

func TestServiceParseFailureCreatesNoResumeOwnershipRowsAndMarksJobFailed(t *testing.T) {
	database := newResumeMigratedSQLiteGormDB(t)
	seedResumeFixture(t, database)
	parser := &fakeParser{errs: []error{resume.ErrParseFailed}}
	service := resume.NewService(resume.NewSQLStore(database), parser, audit.NopRecorder{})

	status, err := service.ImportOne(context.Background(), resume.ImportInput{
		UserID:             "user_owner",
		Channel:            resume.ChannelSocial,
		TargetDepartmentID: "dept_a",
		DataScope:          iam.DataScope{AllDepartments: true},
		File:               pdfFile("bad.pdf"),
	})
	if !errors.Is(err, resume.ErrParseFailed) {
		t.Fatalf("expected parse error, got %v", err)
	}
	if status.Status != "failed" || status.Results[0].ErrorCode != "RESUME_IMPORT_PARSE_FAILED" {
		t.Fatalf("expected failed job result, got %#v", status)
	}
	assertResumeCount(t, database, "resumes", "normalized_name = 'bad'", 0)
	assertResumeCount(t, database, "department_resumes", "resume_id = '"+status.Results[0].ResumeID+"'", 0)
}

func TestServiceBatchImportRecordsPerFileResults(t *testing.T) {
	database := newResumeMigratedSQLiteGormDB(t)
	seedResumeFixture(t, database)
	parser := &fakeParser{
		results: []resume.ParsedResume{parsedResume("孙八")},
		errs:    []error{nil, resume.ErrParseFailed},
	}
	service := resume.NewService(resume.NewSQLStore(database), parser, audit.NopRecorder{})

	status, err := service.ImportBatch(context.Background(), resume.BatchImportInput{
		UserID:             "user_owner",
		Channel:            resume.ChannelSocial,
		TargetDepartmentID: "dept_a",
		DataScope:          iam.DataScope{AllDepartments: true},
		Files:              []resume.ImportFile{pdfFile("sunba.pdf"), pdfFile("failed.pdf")},
	})
	if err != nil {
		t.Fatalf("batch import should preserve per-file failures without failing request: %v", err)
	}
	if status.Summary.Total != 2 || status.Summary.Succeeded != 1 || status.Summary.Failed != 1 {
		t.Fatalf("expected mixed batch summary, got %#v", status.Summary)
	}
	if status.Results[0].Status != "succeeded" || status.Results[1].Status != "failed" {
		t.Fatalf("expected per-file results, got %#v", status.Results)
	}
}

func TestServiceGetJobRejectsDifferentOwner(t *testing.T) {
	database := newResumeMigratedSQLiteGormDB(t)
	seedResumeFixture(t, database)
	service := resume.NewService(resume.NewSQLStore(database), &fakeParser{results: []resume.ParsedResume{parsedResume("周九")}}, audit.NopRecorder{})
	status, err := service.ImportOne(context.Background(), resume.ImportInput{
		UserID:             "user_owner",
		Channel:            resume.ChannelSocial,
		TargetDepartmentID: "dept_a",
		DataScope:          iam.DataScope{AllDepartments: true},
		File:               pdfFile("zhoujiu.pdf"),
	})
	if err != nil {
		t.Fatalf("import one: %v", err)
	}

	_, err = service.GetJob(context.Background(), status.ID, "different_user")
	if !errors.Is(err, resume.ErrJobAccessDenied) {
		t.Fatalf("expected job access denied, got %v", err)
	}
}

func TestServiceWritesAuditForImportAndDeleteWithoutSensitivePayloads(t *testing.T) {
	database := newResumeMigratedSQLiteGormDB(t)
	seedResumeFixture(t, database)
	recorder := &spyAuditRecorder{}
	service := resume.NewService(resume.NewSQLStore(database), &fakeParser{results: []resume.ParsedResume{parsedResume("吴十")}}, recorder)

	if _, err := service.ImportOne(context.Background(), resume.ImportInput{
		UserID:             "user_owner",
		Channel:            resume.ChannelSocial,
		TargetDepartmentID: "dept_a",
		DataScope:          iam.DataScope{AllDepartments: true},
		File:               pdfFile("wushi.pdf"),
	}); err != nil {
		t.Fatalf("import one: %v", err)
	}
	if err := service.Delete(context.Background(), "resume_social_a", scopeFor(iam.ActionDelete, iam.ScopeBranch{DepartmentIDs: []string{"dept_a"}, Channels: []string{"social"}})); err != nil {
		t.Fatalf("delete resume: %v", err)
	}

	assertResumeAudit(t, recorder.events, audit.EventResumeImportSucceeded)
	assertResumeAudit(t, recorder.events, audit.EventResumeDeleted)
	if strings.Contains(fmt.Sprintf("%#v", recorder.events), "rawText") || strings.Contains(fmt.Sprintf("%#v", recorder.events), "profile") {
		t.Fatalf("audit events leaked sensitive payload: %#v", recorder.events)
	}
}

type fakeParser struct {
	calls   int
	results []resume.ParsedResume
	errs    []error
}

func (f *fakeParser) Parse(ctx context.Context, input resume.ParseInput) (resume.ParsedResume, error) {
	f.calls++
	index := f.calls - 1
	if index < len(f.errs) && f.errs[index] != nil {
		return resume.ParsedResume{}, f.errs[index]
	}
	if index < len(f.results) {
		return f.results[index], nil
	}
	return parsedResume(strings.TrimSuffix(input.FileName, ".pdf")), nil
}

type spyAuditRecorder struct {
	events []audit.Event
}

func (s *spyAuditRecorder) Record(ctx context.Context, event audit.Event) error {
	s.events = append(s.events, event)
	return nil
}

func parsedResume(name string) resume.ParsedResume {
	age := 28
	years := 5.0
	return resume.ParsedResume{
		NormalizedName: strings.ToLower(name),
		Name:           name,
		Age:            &age,
		School:         "浙江大学",
		YearsExp:       &years,
		Pos:            "平台工程师",
		Keywords:       []string{"Go"},
		Traits:         []string{"稳定"},
		ExpBase:        80,
		Profile: resume.Profile{
			Basic:          map[string]any{"name": name},
			Education:      []map[string]any{},
			WorkExperience: []map[string]any{},
			Projects:       []map[string]any{},
			Skills:         []string{"Go"},
			Certificates:   []string{},
			RawTextRef:     "raw-text-ref",
		},
	}
}

func pdfFile(name string) resume.ImportFile {
	return resume.ImportFile{FileName: name, ContentType: "application/pdf", Bytes: []byte("%PDF-1.4 test")}
}

func assertResumeAudit(t *testing.T, events []audit.Event, eventType audit.EventType) {
	t.Helper()
	for _, event := range events {
		if event.Type == eventType && event.Resource == "Resume" {
			return
		}
	}
	t.Fatalf("expected audit event %s in %#v", eventType, events)
}
