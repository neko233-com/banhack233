// remotessh: 非交互 SSH 运维工具（密码认证，读命令、跑诊断）。
// 密码只从 SSH_PASS 环境变量读取，不进命令行参数。
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

var diagCommands = []string{
	"hostname; uname -a; echo SSH_CONNECTION=$SSH_CONNECTION",
	"cat /etc/os-release 2>/dev/null | head -3",
	"systemctl is-active fail2ban banhack233 sshguard crowdsec 2>&1; echo ---enabled---; systemctl is-enabled banhack233 fail2ban 2>&1",
	"fail2ban-client status 2>&1",
	"fail2ban-client status 2>/dev/null | sed -n 's/.*Jail list:[[:space:]]*//p' | tr -d ' ' | tr ',' '\\n' | while read j; do if [ -n \"$j\" ]; then echo \"== jail $j ==\"; fail2ban-client status \"$j\" 2>&1; fi; done",
	"banhack233 status 2>&1 | head -150",
	"journalctl -u ssh -u sshd --since '6 hours ago' --no-pager 2>/dev/null | grep -E 'Failed password|Accepted |Invalid user|Ban |Unban' | tail -100",
	"grep -E 'Failed password|Accepted |Invalid user|Ban |Unban' /var/log/auth.log 2>/dev/null | tail -80",
	"iptables -S 2>/dev/null | grep -iE 'DROP|REJECT' | head -40",
	"nft list ruleset 2>/dev/null | grep -iE 'drop|reject' | head -40",
}

func main() {
	host := flag.String("host", "", "SSH 服务器地址")
	port := flag.Int("port", 22, "SSH 端口")
	userName := flag.String("user", "root", "SSH 用户")
	strict := flag.Bool("strict-host-key", false, "未知主机密钥直接失败（默认接受新密钥并记录到 known_hosts）")
	timeout := flag.Duration("timeout", 45*time.Second, "单条命令超时")
	flag.Parse()

	if *host == "" || flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: remotessh -host <addr> [-port 22] [-user root] <diag | exec <command> | put <local> <remote> [mode]>")
		flag.PrintDefaults()
		os.Exit(2)
	}
	password := os.Getenv("SSH_PASS")
	if password == "" {
		fmt.Fprintln(os.Stderr, "SSH_PASS environment variable is required")
		os.Exit(2)
	}

	client, err := dial(*host, *port, *userName, password, *strict)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect %s:%d: %v\n", *host, *port, err)
		os.Exit(1)
	}
	defer client.Close()
	fmt.Fprintf(os.Stderr, "SSH connected to %s:%d\n", *host, *port)

	var cmds []string
	switch flag.Arg(0) {
	case "put":
		if flag.NArg() < 3 {
			fmt.Fprintln(os.Stderr, "usage: remotessh ... put <local> <remote> [mode]")
			os.Exit(2)
		}
		mode := ""
		if flag.NArg() > 3 {
			mode = flag.Arg(3)
		}
		if err := put(client, flag.Arg(1), flag.Arg(2), mode); err != nil {
			fmt.Fprintln(os.Stderr, "put:", err)
			os.Exit(1)
		}
		fmt.Println("uploaded", flag.Arg(2))
		return
	case "diag":
		cmds = diagCommands
	case "exec":
		if flag.NArg() < 2 {
			fmt.Fprintln(os.Stderr, "exec requires a command")
			os.Exit(2)
		}
		cmds = []string{strings.Join(flag.Args()[1:], " ")}
	default:
		fmt.Fprintf(os.Stderr, "unknown mode %q, want diag, exec or put\n", flag.Arg(0))
		os.Exit(2)
	}

	failed := false
	for _, cmd := range cmds {
		out, errOut, code, err := run(client, cmd, *timeout)
		fmt.Printf("=== %s (exit %d) ===\n", cmd, code)
		if out != "" {
			fmt.Print(out)
			if !strings.HasSuffix(out, "\n") {
				fmt.Println()
			}
		}
		if errOut != "" {
			fmt.Print(errOut)
			if !strings.HasSuffix(errOut, "\n") {
				fmt.Println()
			}
		}
		if err != nil {
			fmt.Printf("!! %v\n", err)
			failed = true
		}
		fmt.Println()
	}
	if failed {
		os.Exit(1)
	}
}

// put 通过 SSH 会话的 stdin 上传文件（cat > remote [&& chmod mode remote]），
// 不引入 SFTP 依赖。
func put(client *ssh.Client, local, remote, mode string) error {
	for _, r := range mode {
		if r < '0' || r > '7' {
			return fmt.Errorf("invalid mode %q (expect octal like 755)", mode)
		}
	}
	f, err := os.Open(local)
	if err != nil {
		return err
	}
	defer f.Close()

	quoted := "'" + strings.ReplaceAll(remote, "'", `'\''`) + "'"
	cmd := "cat > " + quoted
	if mode != "" {
		cmd += " && chmod " + mode + " " + quoted
	}
	sess, err := client.NewSession()
	if err != nil {
		return err
	}
	defer sess.Close()
	sess.Stdin = f
	if out, err := sess.CombinedOutput(cmd); err != nil {
		return fmt.Errorf("%s: %w: %s", cmd, err, out)
	}
	return nil
}

// dial 带重试：服务器偶发在握手阶段 EOF，自动重连最多 5 次。
func dial(host string, port int, user, password string, strict bool) (*ssh.Client, error) {
	var lastErr error
	for attempt := 1; attempt <= 5; attempt++ {
		client, err := dialOnce(host, port, user, password, strict)
		if err == nil {
			return client, nil
		}
		lastErr = err
		fmt.Fprintf(os.Stderr, "connect attempt %d/5 failed: %v\n", attempt, err)
		if attempt < 5 {
			time.Sleep(2 * time.Second)
		}
	}
	return nil, lastErr
}

func dialOnce(host string, port int, user, password string, strict bool) (*ssh.Client, error) {
	hostKeyCB, err := hostKeyCallback(strict)
	if err != nil {
		return nil, err
	}
	cfg := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: hostKeyCB,
		Timeout:         15 * time.Second,
	}
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	return ssh.Dial("tcp", addr, cfg)
}

// hostKeyCallback 返回严格校验或 accept-new（未知主机记录、密钥变更拒绝）回调。
func hostKeyCallback(strict bool) (ssh.HostKeyCallback, error) {
	path, err := knownHostsPath()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			return nil, err
		}
		f.Close()
	}
	db, err := knownhosts.New(path)
	if err != nil {
		return nil, err
	}
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		err := db(hostname, remote, key)
		if err == nil || strict {
			return err
		}
		var kErr *knownhosts.KeyError
		if errors.As(err, &kErr) && len(kErr.Want) == 0 {
			// 未知主机：接受并追加到 known_hosts；已知主机密钥不匹配仍拒绝。
			f, ferr := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
			if ferr != nil {
				return ferr
			}
			defer f.Close()
			_, ferr = fmt.Fprintf(f, "%s %s\n", knownhosts.Line([]string{remote.String()}, key), key.Marshal())
			return ferr
		}
		return err
	}, nil
}

func knownHostsPath() (string, error) {
	if u, err := user.Current(); err == nil && u.HomeDir != "" {
		return filepath.Join(u.HomeDir, ".ssh", "known_hosts"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate home dir: %w", err)
	}
	return filepath.Join(home, ".ssh", "known_hosts"), nil
}

func run(client *ssh.Client, cmd string, timeout time.Duration) (stdout, stderr string, code int, err error) {
	sess, err := client.NewSession()
	if err != nil {
		return "", "", -1, err
	}
	defer sess.Close()

	var outBuf, errBuf bytes.Buffer
	sess.Stdout = &outBuf
	sess.Stderr = &errBuf

	done := make(chan error, 1)
	if err = sess.Start(cmd); err != nil {
		return "", "", -1, err
	}
	go func() { done <- sess.Wait() }()

	select {
	case err = <-done:
	case <-time.After(timeout):
		sess.Close()
		return outBuf.String(), errBuf.String(), -1, fmt.Errorf("command timed out after %s", timeout)
	}
	if err != nil {
		var exitErr *ssh.ExitError
		if errors.As(err, &exitErr) {
			return outBuf.String(), errBuf.String(), exitErr.ExitStatus(), nil
		}
		return outBuf.String(), errBuf.String(), -1, err
	}
	return outBuf.String(), errBuf.String(), 0, nil
}
