package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// GetSystemAnnouncements 获取系统公告列表
func GetSystemAnnouncements(c *gin.Context) {
	p, _ := strconv.Atoi(c.Query("p"))
	if p < 0 {
		p = 0
	}
	pageSize, _ := strconv.Atoi(c.Query("page_size"))
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	announcements, err := model.GetAllSystemAnnouncements(p*pageSize, pageSize)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	total, err := model.CountSystemAnnouncements()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    announcements,
		"total":   total,
	})
}

// CreateManualAnnouncement 创建手动公告
func CreateManualAnnouncement(c *gin.Context) {
	var req struct {
		Content string `json:"content" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "参数错误：" + err.Error(),
		})
		return
	}

	err := model.CreateSystemAnnouncement("manual", "", req.Content, true)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "创建公告失败：" + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "公告创建成功",
	})
}

// UpdateSystemAnnouncement 更新系统公告（仅手动添加的可编辑）
func UpdateSystemAnnouncement(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无效的公告ID",
		})
		return
	}

	var req struct {
		Content string `json:"content" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "参数错误：" + err.Error(),
		})
		return
	}

	announcement, err := model.GetSystemAnnouncementById(id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "公告不存在",
		})
		return
	}

	if !announcement.Editable {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "系统自动生成的公告不可编辑",
		})
		return
	}

	announcement.Content = req.Content
	err = announcement.Update()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "更新失败：" + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "更新成功",
	})
}

// DeleteSystemAnnouncement 删除系统公告
func DeleteSystemAnnouncement(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无效的公告ID",
		})
		return
	}

	err = model.DeleteSystemAnnouncement(id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "删除失败：" + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "删除成功",
	})
}

// GetModelTimelines 获取模型时间线（用于排序）
func GetModelTimelines(c *gin.Context) {
	timelines, err := model.GetAllModelTimelines()
	if err != nil {
		common.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    timelines,
	})
}
