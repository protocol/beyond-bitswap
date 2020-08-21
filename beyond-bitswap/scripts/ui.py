import ipywidgets as widgets

class Layout:
    def __init__(self):
        self.testcase = widgets.Text(description="Testcase")
        self.input_data = widgets.Text(description="Input Data Type")
        self.file_size = widgets.Text(description="File Size")
        self.data_dir = widgets.Text(description="Files Directory")
        self.run_count = widgets.IntSlider(description="Run Count", min=1, max=10)

        self.n_nodes = widgets.IntSlider(description="# nodes", min=2, max=50)
        self.n_leechers = widgets.IntSlider(description="# leechers", min=1, max=50)
        self.n_passive = widgets.IntSlider(description="# passive ", min=0, max=10)
        self.max_connection_rate = widgets.IntSlider(description="Max connections (%)", value=100, min=0, max=100)
        self.churn_rate = widgets.IntSlider(description="Churn Rate (%)", min=0, max=100)
        self.isDocker = widgets.Checkbox(value=False,description='Docker Env',disabled=False,indent=False)
        self.bandwidth_mb = widgets.IntSlider(description="Nodes Bandwidth (MB)", value=100, min=0, max=500)
        self.latency_ms = widgets.IntSlider(description="Nodes Latency (ms)", value=10, min=10, max=500)
        self.jitter_pct = widgets.IntSlider(description="Pct Jitter (%)", value=5, min=0, max=100)
        
        self.grid = widgets.GridspecLayout(7, 2, height='300px')


    def show(self):
        self.grid[0, 0] = self.testcase
        self.grid[1, 0] = self.input_data
        self.grid[2, 0] = self.file_size
        self.grid[3, 0] = self.data_dir
        self.grid[4, 0] = self.run_count
        self.grid[0, 1] = self.n_nodes
        self.grid[1, 1] = self.n_leechers
        self.grid[2, 1] = self.n_passive
        self.grid[3, 1] = self.churn_rate
        self.grid[4, 1] = self.isDocker
        self.grid[5, 0] = self.bandwidth_mb
        self.grid[5, 1] = self.latency_ms
        self.grid[6, 1] = self.jitter_pct
        return self.grid


        