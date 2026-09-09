package service

type UserRepository interface {
	GetOneById()
	GetOneByLogin()
}

type UserService interface {
	GetOneById()
	GetOneByLogin()
}

type userService struct {
	repo UserRepository
}

func NewUserService(repo UserRepository) UserService {
	return &userService{
		repo: repo,
	}
}

func (s *userService) GetOneById() {
	s.repo.GetOneById()
}

func (s *userService) GetOneByLogin() {
	s.repo.GetOneByLogin()
}
