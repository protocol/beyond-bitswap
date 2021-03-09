import yaml
import os
import time

TESTGROUND_BIN="testground"
BUILDER = "exec:go"
RUNNER = "local:exec"
BUILDCFG = " --build-cfg skip_runtime_image=true"
BASE_CMD = TESTGROUND_BIN + " run single --plan=testbed --builder=" + \
    BUILDER + " --runner=" + RUNNER + BUILDCFG

# Parses yaml configs
def process_yaml_config(path):
    cmd = BASE_CMD
    with open(path) as file:
        docs = yaml.full_load(file)

    # Parsing use case parameters
    if docs["use_case"]:
        if docs["use_case"]["testcase"]:
            cmd = cmd + " --testcase=" + docs["use_case"]["testcase"]
        if docs["use_case"]["input_data"]:
            cmd = cmd + " -tp input_data=" + docs["use_case"]["input_data"]
        if docs["use_case"]["file_size"]:
            cmd = cmd + " --tp file_size=" + docs["use_case"]["file_size"]
        if docs["use_case"]["run_count"]:
            cmd = cmd + " --tp run_count=" +str(docs["use_case"]["run_count"])
    
    # Parsing network parameters
    if docs["network"]:
        if docs["network"]["n_nodes"]:
            cmd = cmd + " --instances=" + str(docs["network"]["n_nodes"])
        if docs["network"]["n_leechers"]:
            cmd = cmd + " -tp leech_count=" + str(docs["network"]["n_leechers"])  
        if docs["network"]["n_passive"]:
            cmd = cmd + "-tp passive_count=" + str(docs["network"]["n_passive"])  
        if docs["network"]["max_peer_connections"]:
            cmd = cmd + " -tp max_connection_rate=" + str(docs["network"]["max_peer_connections"])  
        # if docs["network"]["churn_rate"]:
        #     cmd = cmd + " -tp churn_rate=" + str(docs["network"]["churn_rate"])

    return cmd

# Parses config from Jupyter layout
def process_layout_config(layout):
    base = BASE_CMD
    if layout.isDocker.value:
        BUILDER = "docker:go"
        RUNNER = "local:docker"
        base = TESTGROUND_BIN + " run single --plan=testbed --builder=" + \
            BUILDER + " --runner=" + RUNNER + BUILDCFG

    if layout.tcpEnabled.value:
        tcpFlag = "true"
    else:
        tcpFlag = "false"

    cmd = base + " --testcase=" + layout.testcase.value + \
        " --instances=" + str(layout.n_nodes.value)
    
    if layout.input_data.value != "":
        cmd = cmd + " -tp input_data=" + layout.input_data.value
    if layout.file_size.value != "":
        cmd = cmd + " -tp file_size=" + layout.file_size.value.replace(" ", "")
    if layout.data_dir.value != "":
        cmd = cmd + " -tp data_dir=" + layout.data_dir.value.replace(" ", "")

    cmd = cmd + " -tp leech_count=" + str(layout.n_leechers.value) + \
        " -tp passive_count=" + str(layout.n_passive.value) + \
        " -tp max_connection_rate=" + str(layout.max_connection_rate.value) + \
        " -tp run_count=" + str(layout.run_count.value) + \
        " -tp bandwidth_mb=" + str(layout.bandwidth_mb.value) + \
        " -tp latency_ms=" + str(layout.latency_ms.value) + \
        " -tp jitter_pct=" + str(layout.jitter_pct.value) + \
        " -tp node_type=" + layout.protocol.value + \
        " -tp enable_tcp=" + tcpFlag

    return cmd

# Testground runner
def runner(cmd):
    print("Running as: ", cmd)
    cmd = cmd + "| tail -n 1 | awk -F 'run is queued with ID:' '{ print $2 }'"
    stream = os.popen(cmd)
    testID = stream.read().replace("\n", "").replace(" ", "")

    print("Received testID: " + testID)

    check_status(testID)

    print("Run for task completed")
    # if len(testID) < 13 and len(testID) > 1:
    #     print("Run completed successfully with testID: %s" % testID)
    # else:
    #     print("There was an error running the testcase. Check daemon.")
    return testID

def check_status(testid):
    cmd = "testground status --task " + testid
    print(cmd)
    cmd = cmd + "| tail -n 2 | awk -F 'Status:' '{ print $2 }'"
    status = "none"
    while status != "complete":
        stream = os.popen(cmd)
        status = stream.read().replace("\n", "").replace(" ", "").strip()
        print("Task status:", status)
        if status == "":
            print("There was an error running the experiment. Check Testground daemon for further details.")
            return
        time.sleep(10)


# Collect data from a testcase
def collect_data(layout, testid, save=False):
    RUNNER = "local:exec"
    if layout.isDocker.value:
        RUNNER = "local:docker"
        
    print("Cleaning previous runs..")
    cmd = "rm -rf results/*"
    print(os.popen(cmd).read())

    print("Collecting data for testid: ", testid)
    cmd = TESTGROUND_BIN + " collect --runner="+RUNNER + " " + testid
    print(os.popen(cmd).read())
    cmd = "tar xzvf %s.tgz && rm %s.tgz && mv %s results/" % (testid, testid, testid)
    print(os.popen(cmd).read())

    if save:
        print("Saving data for testid: %s" % testid)
        cmd = "cp -r results/%s saved/"
        print(os.popen(cmd).read())


# testid = runner(process_config("./config.yaml"))
# collect_data("96c6ff2b6ebf")
# check_status("bub8gid23084pljmerqg")