package service

type DocumentRepository interface {
	GetDocumentById()
	GetDocumentsByUserId()
	CreateDocument()
	UpdateDocument()
	DeleteDocument()
}

type DocumentService interface {
	GetDocumentById()
}

type documentService struct {
	repo DocumentRepository
}

func NewDocumentService(repo DocumentRepository) DocumentService {
	return &documentService{
		repo: repo,
	}
}

func (s *documentService) GetDocumentById() {
	s.repo.GetDocumentById()
}
