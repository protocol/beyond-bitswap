import process
import os
import sys
import utils
from matplotlib.backends.backend_pdf import PdfPages

dir_path = os.path.dirname(os.path.realpath(__file__))
rfc = sys.argv[1]
filename = "/rfc.pdf"
if len(sys.argv) == 3:
    filename = "/" + sys.argv[2] + ".pdf"

print(filename)

with PdfPages(dir_path + "/../../../RFC/"+rfc+filename) as export_pdf:

    agg, testcases = process.aggregate_results(dir_path + "/../../../RFC/results")
    byLatency = process.groupBy(agg, "latencyMS")
    byNodeType = process.groupBy(agg, "nodeType")
    byFileSize = process.groupBy(agg, "fileSize")
    byBandwidth = process.groupBy(agg, "bandwidthMB")
    byTopology = process.groupBy(agg, "topology")

    process.plot_latency(byLatency, byBandwidth, byFileSize)
    export_pdf.savefig()
    process.plot_messages(byFileSize, byTopology)
    export_pdf.savefig()
    # process.plot_bw_overhead(byFileSize, byTopology)
    # export_pdf.savefig()
    # process.plot_througput(byLatency, byBandwidth, byFileSize, byTopology, testcases)
    # export_pdf.savefig()
    process.plot_want_messages(byFileSize, byTopology)
    export_pdf.savefig()
    # process.plot_tcp_latency(byLatency, byBandwidth, byFileSize)
    # export_pdf.savefig()
