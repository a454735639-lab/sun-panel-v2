package models

type File struct {
	BaseModel
	Src      string `json:"src"`
	UserId   uint   `json:"userId"`
	FileName string `json:"fileName" gorm:"varchar(255)"` // 文件名
	Method   int    `gorm:"int(5)" json:"method"`         // 上传方式
	Ext      string `gorm:"varchar(255)" json:"ext"`      // 扩展名
	// IsPublic 是否共享到公共图库。
	//
	// 语义：文件仍然归属上传者（UserId 不变），只是**额外**出现在公共图库里，
	// 供其他账号选用。不采用"把归属改成公共账号"的做法 ——
	// 那样上传者会失去对自己文件的控制权，也无法再取消分享。
	IsPublic int `gorm:"type:tinyint(1);default:0" json:"isPublic"`
}

// 添加一个文件记录
func (m *File) AddFile(userId uint, fileName, ext, src string) (File, error) {
	file := File{
		UserId:   userId,
		FileName: fileName,
		Src:      src,
		Ext:      ext,
	}
	err := Db.Create(&file).Error

	return file, err
}
