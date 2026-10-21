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
	// Usage 用途分类：0 未分类 / 1 壁纸 / 2 图标。
	//
	// 由用户**手动**标记，不做自动推断 ——
	// 按文件名猜不准（"1.png" 是什么？），按尺寸猜不可靠
	// （壁纸和图标尺寸可能重叠），猜错了反而更难找。
	//
	// 分类的价值在于：壁纸与图标是两种不同的使用场景，
	// 混在一起时找图很费劲（图库几十张之后尤其明显）。
	Usage int `gorm:"type:tinyint(1);default:0" json:"usage"`
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
