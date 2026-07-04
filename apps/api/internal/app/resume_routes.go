package app

import (
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/talentpilot/talentpilot/apps/api/internal/http/apperror"
	"github.com/talentpilot/talentpilot/apps/api/internal/iam"
	"github.com/talentpilot/talentpilot/apps/api/internal/resume"
)

type resumeListInput struct {
	Channel string `query:"chan" enum:"social,campus"`
	Search  string `query:"search"`
	Limit   int    `query:"limit" minimum:"1" maximum:"100"`
	Cursor  string `query:"cursor"`
}

type resumeIDInput struct {
	ResumeID string `path:"resumeId"`
}

type jobIDInput struct {
	JobID string `path:"jobId"`
}

type resumeImportInput struct {
	RawBody huma.MultipartFormFiles[resumeImportForm]
}

type resumeImportForm struct {
	File               huma.FormFile `form:"file" contentType:"application/pdf,application/octet-stream" required:"true"`
	Channel            string        `form:"chan" enum:"social,campus" required:"true"`
	TargetDepartmentID string        `form:"targetDepartmentId"`
}

type resumeBatchImportInput struct {
	RawBody huma.MultipartFormFiles[resumeBatchImportForm]
}

type resumeBatchImportForm struct {
	Files              []huma.FormFile `form:"files" contentType:"application/pdf,application/octet-stream" required:"true"`
	Channel            string          `form:"chan" enum:"social,campus" required:"true"`
	TargetDepartmentID string          `form:"targetDepartmentId"`
}

type resumeListOutput struct {
	Body resume.ListResult `json:"body"`
}

type resumeDetailOutput struct {
	Body resume.Detail `json:"body"`
}

type resumeJobOutput struct {
	Body resume.JobStatus `json:"body"`
}

type deleteResumeOutput struct {
	Status int `json:"-"`
}

func registerResumeRoutes(api huma.API, options Options) {
	huma.Register(api, huma.Operation{
		OperationID: "get-resumes",
		Method:      http.MethodGet,
		Path:        "/resumes",
		Summary:     "List resumes",
		Tags:        []string{"resumes"},
		Errors:      []int{http.StatusUnauthorized, http.StatusForbidden},
	}, func(ctx context.Context, input *resumeListInput) (*resumeListOutput, error) {
		principal, scope, err := authorizeRequest(ctx, options, iam.ResourceResume, iam.ActionList)
		if err != nil {
			return nil, err
		}
		service, err := requireResumeService(options.ResumeService)
		if err != nil {
			return nil, err
		}
		getScope, err := scopeForPrincipal(ctx, options, principal, iam.ResourceResume, iam.ActionGet)
		if err != nil {
			getScope = iam.ScopePredicate{}
		}
		deleteScope, err := scopeForPrincipal(ctx, options, principal, iam.ResourceResume, iam.ActionDelete)
		if err != nil {
			deleteScope = iam.ScopePredicate{}
		}
		result, err := service.List(ctx, resume.ListQuery{
			Channel:     resume.Channel(input.Channel),
			Search:      input.Search,
			Limit:       input.Limit,
			Cursor:      input.Cursor,
			Scope:       scope,
			GetScope:    getScope,
			DeleteScope: deleteScope,
		})
		if err != nil {
			return nil, mapResumeError(err)
		}
		return &resumeListOutput{Body: result}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-resume",
		Method:      http.MethodGet,
		Path:        "/resumes/{resumeId}",
		Summary:     "Get resume detail",
		Tags:        []string{"resumes"},
		Errors:      []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound},
	}, func(ctx context.Context, input *resumeIDInput) (*resumeDetailOutput, error) {
		_, scope, err := authorizeRequest(ctx, options, iam.ResourceResume, iam.ActionGet)
		if err != nil {
			return nil, err
		}
		service, err := requireResumeService(options.ResumeService)
		if err != nil {
			return nil, err
		}
		detail, err := service.Get(ctx, input.ResumeID, scope)
		if err != nil {
			return nil, mapResumeError(err)
		}
		return &resumeDetailOutput{Body: detail}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "post-resume-import",
		Method:      http.MethodPost,
		Path:        "/resumes/imports",
		Summary:     "Import one resume",
		Tags:        []string{"resumes"},
		Errors:      []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusUnprocessableEntity},
	}, func(ctx context.Context, input *resumeImportInput) (*resumeJobOutput, error) {
		principal, _, err := authorizeRequest(ctx, options, iam.ResourceResume, iam.ActionCreate)
		if err != nil {
			return nil, err
		}
		if _, _, err := authorizeRequest(ctx, options, iam.ResourceDepartmentResume, iam.ActionCreate); err != nil {
			return nil, err
		}
		service, err := requireResumeService(options.ResumeService)
		if err != nil {
			return nil, err
		}
		form := input.RawBody.Data()
		file, err := importFileFromFormFile(form.File)
		if err != nil {
			return nil, err
		}
		status, err := service.ImportOne(ctx, resume.ImportInput{
			UserID:             principal.User.ID,
			UserName:           principal.User.Name,
			Channel:            resume.Channel(form.Channel),
			TargetDepartmentID: form.TargetDepartmentID,
			DataScope:          principal.DataScope,
			File:               file,
		})
		if err != nil {
			return &resumeJobOutput{Body: status}, mapResumeError(err)
		}
		return &resumeJobOutput{Body: status}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "post-resume-batch-import",
		Method:      http.MethodPost,
		Path:        "/resumes/batch-imports",
		Summary:     "Batch import resumes",
		Tags:        []string{"resumes"},
		Errors:      []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusUnprocessableEntity},
	}, func(ctx context.Context, input *resumeBatchImportInput) (*resumeJobOutput, error) {
		principal, _, err := authorizeRequest(ctx, options, iam.ResourceResume, iam.ActionCreate)
		if err != nil {
			return nil, err
		}
		if _, _, err := authorizeRequest(ctx, options, iam.ResourceDepartmentResume, iam.ActionCreate); err != nil {
			return nil, err
		}
		service, err := requireResumeService(options.ResumeService)
		if err != nil {
			return nil, err
		}
		form := input.RawBody.Data()
		files := make([]resume.ImportFile, 0, len(form.Files))
		for _, formFile := range form.Files {
			file, err := importFileFromFormFile(formFile)
			if err != nil {
				return nil, err
			}
			files = append(files, file)
		}
		status, err := service.ImportBatch(ctx, resume.BatchImportInput{
			UserID:             principal.User.ID,
			UserName:           principal.User.Name,
			Channel:            resume.Channel(form.Channel),
			TargetDepartmentID: form.TargetDepartmentID,
			DataScope:          principal.DataScope,
			Files:              files,
		})
		if err != nil {
			return &resumeJobOutput{Body: status}, mapResumeError(err)
		}
		return &resumeJobOutput{Body: status}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "delete-resume",
		Method:        http.MethodDelete,
		Path:          "/resumes/{resumeId}",
		Summary:       "Delete resume",
		Tags:          []string{"resumes"},
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound},
	}, func(ctx context.Context, input *resumeIDInput) (*deleteResumeOutput, error) {
		_, scope, err := authorizeRequest(ctx, options, iam.ResourceResume, iam.ActionDelete)
		if err != nil {
			return nil, err
		}
		service, err := requireResumeService(options.ResumeService)
		if err != nil {
			return nil, err
		}
		if err := service.Delete(ctx, input.ResumeID, scope); err != nil {
			return nil, mapResumeError(err)
		}
		return &deleteResumeOutput{Status: http.StatusNoContent}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-job",
		Method:      http.MethodGet,
		Path:        "/jobs/{jobId}",
		Summary:     "Get job status",
		Tags:        []string{"jobs"},
		Errors:      []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound},
	}, func(ctx context.Context, input *jobIDInput) (*resumeJobOutput, error) {
		principal, _, err := authorizeRequest(ctx, options, iam.ResourceJob, iam.ActionGet)
		if err != nil {
			return nil, err
		}
		service, err := requireResumeService(options.ResumeService)
		if err != nil {
			return nil, err
		}
		status, err := service.GetJob(ctx, input.JobID, principal.User.ID)
		if err != nil {
			return nil, mapResumeError(err)
		}
		return &resumeJobOutput{Body: status}, nil
	})
}

func requireResumeService(service ResumeService) (ResumeService, error) {
	if service == nil {
		return nil, apperror.NewProblem(apperror.Internal, "简历服务未配置", "", nil)
	}
	return service, nil
}

func authorizeRequest(ctx context.Context, options Options, resource iam.Resource, action iam.Action) (iam.Principal, iam.ScopePredicate, error) {
	authResult, ok := authResultFromContext(ctx)
	if !ok {
		return iam.Principal{}, iam.ScopePredicate{}, apperror.NewProblem(apperror.Unauthenticated, "", "", nil)
	}
	service := options.IAMService
	if service == nil {
		return iam.Principal{}, iam.ScopePredicate{}, apperror.NewProblem(apperror.Internal, "权限服务未配置", "", nil)
	}
	principal, err := service.ResolvePrincipal(ctx, authResult.User.ID)
	if err != nil {
		return iam.Principal{}, iam.ScopePredicate{}, mapIAMError(err)
	}
	decision := service.Can(ctx, principal, resource, action, iam.Target{})
	if !decision.Allowed {
		return iam.Principal{}, iam.ScopePredicate{}, apperror.NewProblem(apperror.PermissionDenied, "", "", map[string]any{"resource": resource, "action": action})
	}
	scope, err := service.Scope(ctx, principal, resource, action)
	if err != nil {
		return iam.Principal{}, iam.ScopePredicate{}, mapIAMError(err)
	}
	return principal, scope, nil
}

func scopeForPrincipal(ctx context.Context, options Options, principal iam.Principal, resource iam.Resource, action iam.Action) (iam.ScopePredicate, error) {
	if options.IAMService == nil {
		return iam.ScopePredicate{}, apperror.NewProblem(apperror.Internal, "权限服务未配置", "", nil)
	}
	decision := options.IAMService.Can(ctx, principal, resource, action, iam.Target{})
	if !decision.Allowed {
		return iam.ScopePredicate{}, apperror.NewProblem(apperror.PermissionDenied, "", "", nil)
	}
	return options.IAMService.Scope(ctx, principal, resource, action)
}

func importFileFromFormFile(file huma.FormFile) (resume.ImportFile, error) {
	if file.File == nil {
		return resume.ImportFile{}, apperror.NewProblem(apperror.ResumeImportEmptyFile, "", "", nil)
	}
	content, err := io.ReadAll(file.File)
	if err != nil {
		return resume.ImportFile{}, apperror.NewProblem(apperror.ResumeImportParseFailed, "", "", nil)
	}
	return resume.ImportFile{FileName: file.Filename, ContentType: file.ContentType, Bytes: content}, nil
}

func mapResumeError(err error) error {
	switch {
	case errors.Is(err, resume.ErrNotFound):
		return apperror.NewProblem(apperror.ResumeNotFound, "", "", nil)
	case errors.Is(err, resume.ErrImportTargetRequired):
		return apperror.NewProblem(apperror.ResumeImportTargetDepartmentRequired, "", "", nil)
	case errors.Is(err, resume.ErrImportTargetInvalid):
		return apperror.NewProblem(apperror.ResumeImportTargetDepartmentInvalid, "", "", nil)
	case errors.Is(err, resume.ErrUnsupportedFileType):
		return apperror.NewProblem(apperror.ResumeImportUnsupportedType, "", "", nil)
	case errors.Is(err, resume.ErrFileTooLarge):
		return apperror.NewProblem(apperror.ResumeImportFileTooLarge, "", "", nil)
	case errors.Is(err, resume.ErrEmptyFile):
		return apperror.NewProblem(apperror.ResumeImportEmptyFile, "", "", nil)
	case errors.Is(err, resume.ErrParseFailed):
		return apperror.NewProblem(apperror.ResumeImportParseFailed, "", "", nil)
	case errors.Is(err, resume.ErrJobNotFound):
		return apperror.NewProblem(apperror.JobNotFound, "", "", nil)
	case errors.Is(err, resume.ErrJobAccessDenied):
		return apperror.NewProblem(apperror.JobAccessDenied, "", "", nil)
	default:
		return apperror.NewProblem(apperror.Internal, "", "", nil)
	}
}
