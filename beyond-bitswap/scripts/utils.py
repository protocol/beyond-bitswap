import yaml
import os

TESTGROUND_BIN="testground"
BUILDER = "exec:go"
RUNNER = "local:exec"
BASE_EXEC = TESTGROUND_BIN + " run single --plan=beyond-bitswap --builder=" + BUILDER + " --runner=" + RUNNER

def process_config(path):
    exec = BASE_EXEC
    with open(path) as file:
        docs = yaml.full_load(file)

    # Parsing use case parameters
    if docs["use_case"]:
        if docs["use_case"]["testcase"]:
            exec = exec + " --testcase=" + docs["use_case"]["testcase"]
        if docs["use_case"]["input_data"]:
            exec = exec + " -tp input_data=" + docs["use_case"]["input_data"]
        if docs["use_case"]["file_size"]:
            exec = exec + " --tp file_size=" + docs["use_case"]["file_size"]
    
    # Parsing network parameters
    if docs["network"]:
        if docs["network"]["n_nodes"]:
            exec = exec + " --instances=" + str(docs["network"]["n_nodes"])
        if docs["network"]["n_leechers"]:
            exec = exec + " -tp leech_count=" + str(docs["network"]["n_leechers"])  
        if docs["network"]["n_passive"]:
            exec = exec + "-tp passive_count=" + str(docs["network"]["n_passive"])  
        if docs["network"]["max_peer_connections"]:
            exec = exec + " -tp max_connection_rate=" + str(docs["network"]["max_peer_connections"])  
        # if docs["network"]["churn_rate"]:
        #     exec = exec + " -tp churn_rate=" + str(docs["network"]["churn_rate"])

    return exec

def runner(cmd):
    print("Running as: ", cmd)
    cmd = cmd + "| tail -n 1 | awk -F 'run with ID: ' '{ print $2 }'"
    stream = os.popen(cmd)
    testID = stream.read()
    return testID

def collect_data(testid):
    print("Collecting data for testid: ", testid)
    cmd = TESTGROUND_BIN + " collect --runner="+RUNNER + " " + testid
    print(os.popen(cmd).read())
    cmd = "tar xzvf %s.tgz && rm %s.tgz && mv %s results/" % (testid, testid, testid)
    print(os.popen(cmd).read())

# testid = runner(process_config("./config.yaml"))
# collect_data(testid)