// Blades 示例：暂停/恢复审批（resume-approve）
//
// 本示例演示如何使用 Blades 的暂停/恢复机制实现人工审批流程。
// 当 Agent 执行到需要人工干预的步骤时，可以暂停执行并等待用户确认，
// 确认后再从暂停点恢复执行。
//
// 适用场景：
// - 请假/报销审批流程
// - 敏感操作确认（删除、转账）
// - 内容审核工作流
// - 需要人工判断的决策点
//
// 核心概念：
// 1. ErrInterrupted：表示执行被中断，等待外部输入
// 2. WithResume：标记是否为恢复执行
// 3. InvocationID：标识一次完整的执行流程
// 4. Session State：在暂停期间保存状态
//
// 使用方法：
// go run main.go
// 注意：需要设置 OPENAI_MODEL 和 OPENAI_API_KEY 环境变量
package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/contrib/openai"
	"github.com/go-kratos/blades/tools"
)

// 常量定义
const (
	leaveApprovalKey    = "leave_approval" // Session 中存储请假审批状态的键
	defaultLeaveRequest = "My name is Alice. I need leave from 2026-03-12 to 2026-03-14 because I have a fever."
)

// LeaveApprovalRequest 表示请假申请请求结构
type LeaveApprovalRequest struct {
	EmployeeName string `json:"employee_name" jsonschema:"Employee name"`
	StartDate    string `json:"start_date" jsonschema:"Leave start date in YYYY-MM-DD format"`
	EndDate      string `json:"end_date" jsonschema:"Leave end date in YYYY-MM-DD format"`
	Reason       string `json:"reason" jsonschema:"Reason for the leave request"`
}

// LeaveApprovalResult 表示请假审批结果
type LeaveApprovalResult struct {
	Approved bool `json:"approved" jsonschema:"Whether the leave request is approved"`
}

// LeaveApprovalState 表示请假审批的中间状态
type LeaveApprovalState struct {
	Request  LeaveApprovalRequest // 请假申请详情
	Decision string               // 审批决定（"approve" 或 "reject"）
}

// requestLeaveApproval 是请假工具的处理函数
// 这个函数会被 Agent 调用，当需要审批时返回 ErrInterrupted 暂停执行
func requestLeaveApproval(ctx context.Context, req LeaveApprovalRequest) (LeaveApprovalResult, error) {
	// 从上下文中获取会话
	session, ok := blades.FromSessionContext(ctx)
	if !ok {
		return LeaveApprovalResult{}, blades.ErrNoSessionContext
	}

	// 从会话中获取当前状态
	state, _ := session.State()[leaveApprovalKey].(LeaveApprovalState)
	// 更新状态，保存请假申请详情
	state.Request = req
	session.SetState(leaveApprovalKey, state)

	// 如果还没有审批决定，返回中断错误
	// 这会让 Agent 暂停执行，等待用户审批
	if state.Decision == "" {
		return LeaveApprovalResult{}, blades.ErrInterrupted
	}

	// 如果已经有审批决定，返回结果
	return LeaveApprovalResult{
		Approved: state.Decision == "approve",
	}, nil
}

// promptApproval 是交互式审批函数
// 它显示请假申请详情并等待用户输入审批决定
func promptApproval(session blades.Session) error {
	// 从会话中获取审批状态
	state, ok := session.State()[leaveApprovalKey].(LeaveApprovalState)
	if !ok {
		return fmt.Errorf("pending leave request not found in session")
	}
	req := state.Request
	reader := bufio.NewReader(os.Stdin)

	// 显示请假申请详情
	fmt.Println("Leave request is waiting for human approval:")
	fmt.Printf("  Employee: %s\n", req.EmployeeName)
	fmt.Printf("  Start:    %s\n", req.StartDate)
	fmt.Printf("  End:      %s\n", req.EndDate)
	fmt.Printf("  Reason:   %s\n", req.Reason)

	// 循环等待有效输入
	for {
		fmt.Print("Decision [approve/reject]: ")
		decision, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		decision = strings.TrimSpace(decision)

		// 验证输入
		if decision != "approve" && decision != "reject" {
			fmt.Println("Please enter either approve or reject.")
			continue
		}

		// 保存审批决定到会话状态
		state.Decision = decision
		session.SetState(leaveApprovalKey, state)
		return nil
	}
}

func main() {
	// 步骤 1: 创建请假审批工具
	approvalTool, err := tools.NewFunc(
		"request_leave_approval",
		"Submit a leave request for human approval. Always call this tool before answering a leave request.",
		requestLeaveApproval,
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 2: 创建 OpenAI 模型
	model := openai.NewModel(os.Getenv("OPENAI_MODEL"), openai.Config{
		APIKey: os.Getenv("OPENAI_API_KEY"),
	})

	// 步骤 3: 创建 Agent
	// Agent 的指令告诉它如何处理请假申请
	agent, err := blades.NewAgent(
		"LeaveApprovalAgent",
		blades.WithModel(model),
		blades.WithInstruction(`You are an HR leave assistant.
For every leave request:
1. Extract employee_name, start_date, end_date, and reason.
2. Call the request_leave_approval tool exactly once.
3. After the tool returns, tell the user whether the leave was approved or rejected.`),
		blades.WithTools(approvalTool),
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 4: 准备输入和会话
	input := blades.UserMessage(defaultLeaveRequest)
	ctx := context.Background()
	session := blades.NewSession()
	invocationID := "leave-approval-001" // 唯一标识这次执行

	// 步骤 5: 第一次运行（会触发审批中断）
	runner := blades.NewRunner(agent)
	output, err := runner.Run(
		ctx,
		input,
		blades.WithSession(session),
		blades.WithInvocationID(invocationID),
	)

	// 预期会收到 ErrInterrupted 错误，表示等待审批
	if !errors.Is(err, blades.ErrInterrupted) {
		log.Fatal(err)
	}
	log.Println("leave request paused, waiting for human approval")

	// 步骤 6: 等待用户审批
	if err := promptApproval(session); err != nil {
		log.Fatal(err)
	}

	// 步骤 7: 恢复执行
	// WithResume(true) 标记这是恢复执行
	// Agent 会从暂停点继续，工具会返回审批结果
	output, err = runner.Run(
		ctx,
		input,
		blades.WithResume(true), // 标记为恢复执行
		blades.WithSession(session),
		blades.WithInvocationID(invocationID),
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 8: 输出最终结果
	log.Println(output.Author, output.Text())

	// 预期流程：
	// 1. Agent 提取请假信息
	// 2. Agent 调用 request_leave_approval 工具
	// 3. 工具返回 ErrInterrupted，执行暂停
	// 4. 程序显示审批界面，用户输入 approve/reject
	// 5. 恢复执行，工具返回审批结果
	// 6. Agent 根据审批结果生成最终回答
	//
	// 扩展提示：
	// 1. 持久化状态：
	//    - 将 Session 状态保存到数据库
	//    - 使用 HTTP 回调进行远程审批
	//    - 支持超时自动处理
	//
	// 2. 多级审批：
	//    - 可以添加多个审批节点
	//    - 支持会签/或签模式
	//
	// 3. 审批通知：
	//    - 发送邮件/短信通知审批人
	//    - 集成 Slack/钉钉等 IM 工具
}
