#define _GNU_SOURCE
#include <errno.h>
#include <fcntl.h>
#include <sched.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/types.h>
#include <sys/wait.h>
#include <unistd.h>

int nsenter(pid_t nspid)
{
	char ns_ipc[64];
	char ns_mnt[64];
	char ns_pid[64];
	char root[64];
	int fd_ipc, fd_mnt, fd_pid, fd_root;

	sprintf(ns_ipc, "/proc/%d/ns/ipc", nspid);
	sprintf(ns_mnt, "/proc/%d/ns/mnt", nspid);
	sprintf(ns_pid, "/proc/%d/ns/pid", nspid);
	sprintf(root, "/proc/%d/root", nspid);

	fd_ipc = open(ns_ipc, O_RDONLY);
	if (fd_ipc == -1) {
		fprintf(stderr, "failed to open %s: %s\n", ns_ipc, strerror(errno));
		return -1;
	}
	fd_mnt = open(ns_mnt, O_RDONLY);
	if (fd_mnt == -1) {
		fprintf(stderr, "failed to open %s: %s\n", ns_mnt, strerror(errno));
		return -1;
	}
	fd_pid = open(ns_pid, O_RDONLY);
	if (fd_pid == -1) {
		fprintf(stderr, "failed to open %s: %s\n", ns_pid, strerror(errno));
		return -1;
	}
	fd_root = open(root, O_RDONLY);
	if (fd_root == -1) {
		fprintf(stderr, "failed to open %s: %s\n", root, strerror(errno));
		return -1;
	}

	if (setns(fd_ipc, CLONE_NEWIPC) == -1) {
		perror("setns ipc failed");
		return -1;
	}
	if (setns(fd_mnt, CLONE_NEWNS) == -1) {
		perror("setns mnt failed");
		return -1;
	}
	if (setns(fd_pid, CLONE_NEWPID) == -1) {
		perror("setns pid failed");
		return -1;
	}
	if (fchdir(fd_root) == -1) {
		perror("fchdir failed");
		return -1;
	}
	if (chroot(".") == -1) {
		perror("chroot failed");
		return -1;
	}
	return 0;
}

void wait4child(pid_t pid)
{
	pid_t w;
	int wstatus = 0;

	while (1) {
		w = waitpid(pid, &wstatus, WUNTRACED | WCONTINUED);
		if (w == -1) {
			perror("waitpid failed");
			exit(1);
		}
		if (WIFEXITED(wstatus)) {
			exit(WEXITSTATUS(wstatus));
		}
		if (WIFSIGNALED(wstatus)) {
			// bash like: signal number + 128
			exit(WTERMSIG(wstatus) + 128);
		}
	}
}

int main(int argc, char **argv)
{
	pid_t pid, nspid;

	if (argc < 3) {
		fprintf(stderr, "Usage: %s <nspid> <cmd> <arguments>\n", argv[0]);
		exit(1);
	}
	if (sscanf(argv[1], "%d", &nspid) != 1) {
		fprintf(stderr, "invalid number %s", argv[1]);
		exit(1);
	}

	if (nsenter(nspid) != 0) {
		exit(1);
	}

	switch (pid = fork()) {
	case -1:
		perror("fork failed\n");
		exit(1);
	case 0:
		// child
		execvpe(argv[2], argv + 2, environ);
		fprintf(stderr, "exec '%s' failed: %s\n", argv[2], strerror(errno));
		// bash like exit status
		if (errno == 13) {
			exit(126);
		} else {
			exit(127);
		}
	default:
		// parent
		wait4child(pid);
	}
}
