import process
import os
import sys
import utils
from matplotlib.backends.backend_pdf import PdfPages

dir_path = os.path.dirname(os.path.realpath(__file__))
print(dir_path)
name = sys.argv[1]
filename = "/" + name + ".pdf"

print(filename)

with PdfPages(dir_path + "/../../compositions/results/"+filename) as export_pdf:

    agg, testcases = process.aggregate_results(dir_path + "/../../compositions/results")
    byLatency = process.groupBy(agg, "latencyMS")
    byNodeType = process.groupBy(agg, "nodeType")
    byFileSize = process.groupBy(agg, "fileSize")
    byBandwidth = process.groupBy(agg, "bandwidthMB")
    byTopology = process.groupBy(agg, "topology")

    process.plot_latency_no_comparision(byLatency, byBandwidth, byFileSize)
    export_pdf.savefig()