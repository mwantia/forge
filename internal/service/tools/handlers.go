package tools

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// executeBody is the request body for tool execution.
// Tool name and call ID come from URL params; only arguments are supplied in the body.
type executeBody struct {
	Arguments map[string]any `json:"arguments"`
}

// handleListTools godoc
//
//	@Description	Returns definitions for all registered tools across all namespaces
func (s *ToolsService) handleListTools() gin.HandlerFunc {
	return func(c *gin.Context) {
		definitions, err := s.GetAllToolDefinitions()
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"tools": definitions})
	}
}

// handleListToolsByNamespace godoc
//
//	@Description	Returns definitions for all tools in the given namespace
func (s *ToolsService) handleListToolsByNamespace() gin.HandlerFunc {
	return func(c *gin.Context) {
		namespace := c.Param("namespace")
		definitions, err := s.GetToolDefinitionsByNamespace(namespace)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"tools": definitions})
	}
}

// handleGetTool godoc
//
//	@Description	Returns the definition for a single tool by namespace and name
func (s *ToolsService) handleGetTool() gin.HandlerFunc {
	return func(c *gin.Context) {
		namespace := c.Param("namespace")
		name := c.Param("name")

		definition, err := s.GetToolDefinition(namespace, name)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, definition)
	}
}

// handleExecuteTool godoc
//
//	@Description	Executes a tool by namespace and name with the provided arguments
func (s *ToolsService) handleExecuteTool() gin.HandlerFunc {
	return func(c *gin.Context) {
		namespace := c.Param("namespace")
		name := c.Param("name")

		var body executeBody
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		resp, err := s.ExecuteTool(c.Request.Context(), namespace, name, body.Arguments)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, resp)
	}
}

// handleExecuteToolWithCallID godoc
//
//	@Description	Executes a tool by namespace and name, associating the result with the given call ID
func (s *ToolsService) handleExecuteToolWithCallID() gin.HandlerFunc {
	return func(c *gin.Context) {
		namespace := c.Param("namespace")
		name := c.Param("name")
		callID := c.Param("callid")

		var body executeBody
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		resp, err := s.ExecuteToolWithCallID(c.Request.Context(), namespace, name, body.Arguments, callID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, resp)
	}
}
