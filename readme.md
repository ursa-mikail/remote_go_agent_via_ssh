# Remote Go Agent via SSH

This repository demonstrates different approaches to building a **remote agent in Go** that communicates and executes commands via **SSH**.  
It progresses from simple proof-of-concept examples to a structured, modular framework.

## 📂 Repository Structure

### 1. [`basic/`](./basic)
A minimal starting point showing how to establish SSH connectivity and run commands remotely.

- Connects to an SSH server
- Executes commands
- Prints results

---

### 2. [`ssh_capsule_load_and_run/`](./ssh_capsule_load_and_run)
Introduces the concept of **capsules** (packaged payloads) that can be transferred and executed remotely.

- Load a payload (e.g., script or binary)
- Send via SSH
- Execute and (optionally) clean up

---

### 3. [`ssh_relay_and_remote_control/`](./ssh_relay_and_remote_control)
Adds a **relay and control layer** for managing multiple agents through a central relay server.

- Relay handles connections
- Remote agents connect back
- Centralized remote control

---

### 4. [`structured/`](./structured)
This folder contains a **modular remote Go agent** that demonstrates retrieving `key*.json` file from a remote system via SSH 1 at a time upon specific request.  
It provides a more organized structure for automation, monitoring, and remote task execution.

- Connects to a remote system over SSH
- Retrieves **one `key.json` file at a time**
- Supports automation of remote tasks via `Taskfile.yml`
- Provides modular libraries for logging, monitoring, execution, and file upload
- Easy to extend for additional remote operations

---


