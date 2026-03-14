// burnscope frontend - Wails bindings

// Wails runtime
let runtime;

// State
let state = {
    mode: 'record',  // 'record' | 'compare'
    isRunning: false,
    port: '',
    baud: 115200
};

// DOM Elements
const portSelect = document.getElementById('port-select');
const baudSelect = document.getElementById('baud-select');
const actionBtn = document.getElementById('action-btn');
const statusDot = document.getElementById('status-dot');
const statusText = document.getElementById('status-text');
const recordsDiv = document.getElementById('records');
const emptyState = document.getElementById('empty-state');
const recordsTitle = document.getElementById('records-title');
const statMatch = document.getElementById('stat-match');
const statDiff = document.getElementById('stat-diff');
const ptyPath = document.getElementById('pty-path');
const modeTabs = document.querySelectorAll('.mode-tab');

// Initialize
document.addEventListener('DOMContentLoaded', async () => {
    // Get Wails runtime
    runtime = window['@wailsapp/runtime'];
    
    // Load ports
    await loadPorts();
    
    // Setup mode tabs
    modeTabs.forEach(tab => {
        tab.addEventListener('click', () => switchMode(tab.dataset.mode));
    });
    
    // Setup action button
    actionBtn.addEventListener('click', toggleAction);
    
    // Refresh ports periodically
    setInterval(loadPorts, 3000);
});

// Load serial ports
async function loadPorts() {
    try {
        const ports = await window.go.main.App.ListSerialPorts();
        portSelect.innerHTML = '';
        
        if (ports.length === 0) {
            portSelect.innerHTML = '<option value="">未检测到串口</option>';
            return;
        }
        
        ports.forEach(port => {
            const opt = document.createElement('option');
            opt.value = port;
            opt.textContent = port;
            portSelect.appendChild(opt);
        });
        
        if (state.port && ports.includes(state.port)) {
            portSelect.value = state.port;
        }
    } catch (e) {
        console.error('Failed to load ports:', e);
        portSelect.innerHTML = '<option value="">加载失败</option>';
    }
}

// Switch mode
function switchMode(mode) {
    if (state.isRunning) return;
    
    state.mode = mode;
    
    modeTabs.forEach(tab => {
        tab.classList.toggle('active', tab.dataset.mode === mode);
    });
    
    if (mode === 'record') {
        actionBtn.textContent = '开始录制';
        recordsTitle.textContent = '交互记录';
        ptyPath.textContent = '';
    } else {
        actionBtn.textContent = '开始对比';
        recordsTitle.textContent = '对比记录';
    }
    
    updateStatus('idle');
}

// Toggle action
async function toggleAction() {
    if (state.isRunning) {
        await stop();
    } else {
        await start();
    }
}

// Start recording/comparing
async function start() {
    const port = portSelect.value;
    const baud = parseInt(baudSelect.value);
    
    if (state.mode === 'record') {
        if (!port) {
            alert('请选择串口');
            return;
        }
        
        try {
            await window.go.main.App.StartRecording(port, baud);
            state.isRunning = true;
            actionBtn.textContent = '停止录制';
            actionBtn.classList.add('stop');
            updateStatus('recording');
            clearRecords();
        } catch (e) {
            alert('启动录制失败: ' + e);
        }
    } else {
        try {
            const pty = await window.go.main.App.StartCompare();
            state.isRunning = true;
            actionBtn.textContent = '停止对比';
            actionBtn.classList.add('stop');
            updateStatus('comparing');
            ptyPath.textContent = '虚拟串口: ' + pty;
            clearRecords();
        } catch (e) {
            alert('启动对比失败: ' + e);
        }
    }
}

// Stop recording/comparing
async function stop() {
    try {
        if (state.mode === 'record') {
            await window.go.main.App.StopRecording();
        } else {
            await window.go.main.App.StopCompare();
        }
        
        state.isRunning = false;
        
        if (state.mode === 'record') {
            actionBtn.textContent = '开始录制';
        } else {
            actionBtn.textContent = '开始对比';
            ptyPath.textContent = '';
        }
        actionBtn.classList.remove('stop');
        updateStatus('idle');
    } catch (e) {
        console.error('Stop failed:', e);
    }
}

// Update status indicator
function updateStatus(status) {
    statusDot.className = 'status-dot';
    
    switch (status) {
        case 'recording':
            statusDot.classList.add('recording');
            statusText.textContent = '录制中';
            break;
        case 'comparing':
            statusDot.classList.add('comparing');
            statusText.textContent = '对比中';
            break;
        default:
            statusText.textContent = '就绪';
    }
}

// Clear records
function clearRecords() {
    recordsDiv.innerHTML = '';
    emptyState.style.display = 'flex';
    recordsDiv.appendChild(emptyState);
    statMatch.textContent = '0';
    statDiff.textContent = '0';
}

// Add record pair (baseline + compare)
function addRecordPair(index, baseline, compare, match) {
    emptyState.style.display = 'none';
    
    const pair = document.createElement('div');
    pair.className = 'record-pair';
    
    // Baseline row
    const baselineRow = createRecordRow('基准', baseline, null);
    pair.appendChild(baselineRow);
    
    // Compare row (if in compare mode)
    if (compare) {
        const compareRow = createRecordRow('对比', compare, match);
        pair.appendChild(compareRow);
    }
    
    // Divider
    const divider = document.createElement('div');
    divider.className = 'divider';
    pair.appendChild(divider);
    
    recordsDiv.appendChild(pair);
    recordsDiv.scrollTop = recordsDiv.scrollHeight;
}

// Create a single record row
function createRecordRow(label, record, match) {
    const row = document.createElement('div');
    row.className = 'record-row baseline';
    
    const dirClass = record.direction.toLowerCase();
    const matchIcon = match === true ? '✓' : (match === false ? '✗' : '');
    const matchClass = match === true ? 'match' : (match === false ? 'diff' : '');
    
    row.innerHTML = `
        <span class="label">${label}</span>
        <span class="dir ${dirClass}">${record.direction}</span>
        <span class="name">${record.name}</span>
        <span class="data">${formatHex(record.raw_data)}</span>
        <span class="result ${matchClass}">${matchIcon}</span>
    `;
    
    return row;
}

// Format hex data
function formatHex(data) {
    if (!data) return '';
    
    // If it's a base64 string, decode it
    if (typeof data === 'string') {
        try {
            // Assume it's already hex formatted or convert
            return data.length > 64 ? data.substring(0, 64) + '...' : data;
        } catch (e) {
            return data;
        }
    }
    
    // If it's an array
    if (Array.isArray(data)) {
        const hex = data.map(b => b.toString(16).padStart(2, '0')).join(' ');
        return hex.length > 64 ? hex.substring(0, 64) + '...' : hex;
    }
    
    return String(data);
}

// Update stats
function updateStats(matched, diff) {
    statMatch.textContent = matched;
    statDiff.textContent = diff;
}

// Expose functions for backend events
window.burnscope = {
    addRecordPair,
    updateStats
};