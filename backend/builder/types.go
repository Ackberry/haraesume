package builder

import "fmt"

type BuilderRequest struct {
	Heading    HeadingData  `json:"heading"`
	Education  []Education  `json:"education"`
	Experience []Experience `json:"experience"`
	Projects   []Project    `json:"projects"`
	Skills     SkillsData   `json:"skills"`
	Leadership []Leadership `json:"leadership"`
}

type HeadingData struct {
	Name      string `json:"name"`
	LinkedIn  string `json:"linkedin"`
	GitHub    string `json:"github"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	Portfolio string `json:"portfolio"`
}

type Education struct {
	School     string `json:"school"`
	Graduation string `json:"graduation"`
	Degree     string `json:"degree"`
	Location   string `json:"location"`
	Coursework string `json:"coursework"`
}

type Experience struct {
	Company  string   `json:"company"`
	Dates    string   `json:"dates"`
	Title    string   `json:"title"`
	Location string   `json:"location"`
	Bullets  []string `json:"bullets"`
}

type Project struct {
	Name      string   `json:"name"`
	Link      string   `json:"link"`
	TechStack string   `json:"techStack"`
	Bullets   []string `json:"bullets"`
}

type SkillsData struct {
	Languages      string `json:"languages"`
	Frameworks     string `json:"frameworks"`
	Databases      string `json:"databases"`
	Infrastructure string `json:"infrastructure"`
}

type Leadership struct {
	Organization string `json:"organization"`
	Dates        string `json:"dates"`
	Title        string `json:"title"`
	Location     string `json:"location"`
}

func validateRequest(req *BuilderRequest) error {
	if req.Heading.Name == "" {
		return fmt.Errorf("heading name is required")
	}
	if req.Heading.Email == "" {
		return fmt.Errorf("heading email is required")
	}
	return nil
}
