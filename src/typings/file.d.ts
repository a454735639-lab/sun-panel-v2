declare namespace File {

	interface Info extends Common.InfoBase {
		src: string
		userId: number
		fileName: string
		method: number
		/**
		 * 扩展名。**可能不存在** —— 后端 GetList 是手工拼返回结构，
		 * 曾漏掉该字段；而上传接口返回的对象也没有它。
		 * 需要判断文件类型时请以文件名后缀为准（可选地拿它做交叉判断）。
		 */
		ext?: string
		/** 服务端存储路径 */
		path?: string
		/** 是否已共享到公共图库（1 是 / 0 否） */
		isPublic?: number
		/** 用途分类：0 未分类 / 1 壁纸 / 2 图标 */
		usage?: number
	}


}