// burnscope frontend - 状态驱动渲染

import * as App from '../wailsjs/go/main/App.js';
import { EventsOn } from '../wailsjs/runtime/runtime.js';

// ============ State ============
const state = {
    mode: '',           // '', 'record', 'compare'
    status: '就绪',
    
    upperPort: '', 
    lowerPorts: [],     // [{portPath:'', portType:''}]
    
    baseline: [],       // [{index, dir, data, size}]
    actual: [],         // [{index, dir, data, size, match}]
    
    stats: { tx: 0, rx: 0, matched: 0, diff: 0 }
};

// ============ Persist ============
const STORAGE_KEY = 'burnscope-baseline';

function saveBaseline() {
    try {
        localStorage.setItem(STORAGE_KEY, JSON.stringify(state.baseline));
    } catch (e) {
        console.warn('Failed to save baseline:', e);
    }
}

function loadBaseline() {
    try {
        const data = localStorage.getItem(STORAGE_KEY);
        if (data) {
            state.baseline = JSON.parse(data);
            // 计算统计
            state.stats.tx = state.baseline.filter(r => r.dir === 'TX').length;
            state.stats.rx = state.baseline.filter(r => r.dir === 'RX').length;
        }
    } catch (e) {
        console.warn('Failed to load baseline:', e);
    }
}

function clearBaseline() {
    state.baseline = [];
    state.actual = [];
    state.stats = { tx: 0, rx: 0, matched: 0, diff: 0 };
    localStorage.removeItem(STORAGE_KEY);
}

// ============ Reducers ============
function setState(updates) {
    Object.assign(state, updates);
    render();
}

function addBaseline(record) {
    state.baseline.push(record);
    if (record.dir === 'TX') state.stats.tx++;
    else state.stats.rx++;
    saveBaseline();
    render();
}

function addActual(record) {
    state.actual.push(record);
    if (record.match === true) state.stats.matched++;
    else if (record.match === false) state.stats.diff++;
    render();
}

// ============ Effects ============
async function initPorts() {
    const result = await App.Init();
    if (result.error) {
        setState({ status: result.error });
        return;
    }
    
    // 加载持久化的 baseline
    loadBaseline();
    
    setState({
        mode: result.mode || 'record',  // 默认录制模式
        status: '录制中',
        upperPort: result.upperPort,
        lowerPorts: result.lowerPorts || [],
        baseline: state.baseline,
        stats: state.stats
    });
}

async function loadPhysicalPorts() {
    const result = await App.RefreshPorts();
    setState({ lowerPorts: result.lowerPorts || [] });
}

async function startRecord() {
    const result = await App.StartRecord();
    if (result.error) {
        alert(result.error);
        return false;
    }
    
    // 保留已有 baseline，继续追加录制
    setState({
        mode: 'record',
        status: '录制中',
        upperPort: result.upperPort || state.upperPort,
        baseline: result.baseline || state.baseline,
        actual: [],
        stats: result.stats || state.stats
    });
    return true;
}

async function startCompare() {
    const result = await App.StartCompare();
    
    // 后端检查 baseline 是否为空
    if (result.mode !== 'compare') {
        alert('没有基准数据，请先录制');
        return false;
    }
    
    // 使用后端返回的状态
    setState({
        mode: 'compare',
        status: '对比中',
        upperPort: result.upperPort || state.upperPort,
        baseline: result.baseline || state.baseline,
        actual: [],
        stats: result.stats || { tx: 0, rx: 0, matched: 0, diff: 0 }
    });
    return true;
}

async function stop() {
    await App.Stop();
    
    // 录制完成后保存 baseline
    if (state.mode === 'record' && state.baseline.length > 0) {
        saveBaseline();
    }
    
    setState({
        mode: '',
        status: '就绪'
    });
}

async function clear() {
    const result = await App.Clear();
    clearBaseline();
    setState({
        mode: result.mode || 'record',
        status: '录制中',
        baseline: [],
        actual: [],
        stats: { tx: 0, rx: 0, matched: 0, diff: 0 }
    });
}

// ============ Event Handlers ============
function onRecord(data) {
    if (state.mode === 'record') {
        // 录制模式：添加到 baseline
        addBaseline({
            index: state.baseline.length,
            dir: data.dir,
            data: data.data,
            size: data.size
        });
    } else if (state.mode === 'compare') {
        // 对比模式：添加到 actual
        addActual({
            index: data.index,
            dir: data.dir || 'TX',
            data: data.data,
            size: data.size,
            match: data.match
        });
    }
}

function onStats(data) {
    state.stats.matched = data.matched;
    state.stats.diff = data.diff;
    render();
}

function onError(msg) {
    alert('错误: ' + msg);
}

// ============ Render ============
function render() {
    // 状态
    document.getElementById('status-text').textContent = state.status;
    document.getElementById('status-dot').className = 'status-dot' + (state.mode ? ' running' : '');
    
    // 模式选择
    document.querySelectorAll('input[name="mode"]').forEach(r => {
        r.checked = r.value === state.mode;
    });
    
    // 端口
    document.getElementById('upper-port').textContent = state.upperPort;
    
    // 下位端口下拉框
    const select = document.getElementById('lower-port');
    select.innerHTML = state.lowerPorts.map(p => 
        `<option value="${p.portType}:${p.portPath}">${p.portPath}（${p.portType === 'virtual' ? '虚拟串口' : '物理串口'}）</option>`
    ).join('');
    
    // 统计
    document.getElementById('stat-tx').textContent = state.stats.tx;
    document.getElementById('stat-rx').textContent = state.stats.rx;
    document.getElementById('stat-match').textContent = state.stats.matched;
    document.getElementById('stat-diff').textContent = state.stats.diff;
    
    // 数据流 - 统一渲染逻辑（录制和对比都是基准+实际两行结构）
    const container = document.getElementById('records');
    
    if (state.baseline.length === 0) {
        container.innerHTML = '<div class="empty-state"><p>' + 
            (state.mode === 'record' ? '等待数据...' : 
             state.mode === 'compare' ? '没有基准数据' : '请选择模式开始') + 
            '</p></div>';
        return;
    }
    
    // 统一渲染：每条 baseline 记录都有对应的位置
    // actual 通过 index 与 baseline 对齐
    let html = '';
    
    state.baseline.forEach((baseRecord, i) => {
        // 基准行（录制和对比都显示）
        html += renderRow(baseRecord, 'baseline');
        
        // 实际行 - 通过 index 精确匹配
        if (state.mode === 'compare') {
            const actualRecord = state.actual.find(r => r.index === i);
            if (actualRecord) {
                html += renderRow(actualRecord, 'actual');
            } else {
                html += renderRow({ dir: baseRecord.dir }, 'actual-empty');
            }
        } else if (state.mode === 'record') {
            // 录制模式：显示空的实际行占位
            html += renderRow({ dir: baseRecord.dir }, 'actual-empty');
        }
    });
    
    container.innerHTML = html;
}

function renderRow(r, type) {
    if (type === 'actual-empty') {
        return `<div class="record-row actual">
            <span class="label">实际</span>
            <span class="dir ${(r.dir || '').toLowerCase()}"></span>
            <span class="size"></span>
            <span class="data"></span>
        </div>`;
    }
    
    const dirClass = (r.dir || '').toLowerCase();
    const matchClass = r.match === true ? 'match' : (r.match === false ? 'diff' : '');
    const icon = r.match === true ? '✓' : (r.match === false ? '✗' : '');
    const labelClass = type === 'baseline' ? 'baseline' : 'actual';
    
    return `<div class="record-row ${labelClass} ${matchClass}">
        <span class="label">${type === 'baseline' ? '基准' : '实际'}</span>
        <span class="dir ${dirClass}">${r.dir || ''}</span>
        <span class="size">${r.size ? r.size + 'B' : ''}</span>
        <span class="data">${truncateHex(r.data, 48)}</span>
        ${icon ? `<span class="result ${matchClass}">${icon}</span>` : ''}
    </div>`;
}

// ============ Helpers ============
function truncateHex(hex, maxLen) {
    if (!hex || hex.length <= maxLen) return hex || '';
    return hex.substring(0, maxLen) + '...';
}

// ============ Actions ============
window.onLowerPortChange = function() {
    // 不需要额外处理
};

window.onModeChange = async function(newMode) {
    // 相同模式则停止
    if (state.mode === newMode) {
        await stop();
        return;
    }
    
    // 先停止当前模式（保留 baseline）
    if (state.mode) {
        await stop();
    }
    
    // 启动新模式
    if (newMode === 'record') {
        await startRecord();
    } else if (newMode === 'compare') {
        await startCompare();
    }
};

window.onClear = clear;

window.copyPort = function() {
    if (state.upperPort && navigator.clipboard) {
        navigator.clipboard.writeText(state.upperPort);
        const el = document.getElementById('upper-port');
        el.style.color = '#238636';
        setTimeout(() => el.style.color = '', 200);
    }
};

// ============ Init ============
window.addEventListener('load', async () => {
    EventsOn('record', onRecord);
    EventsOn('stats', onStats);
    EventsOn('error', onError);
    
    await initPorts();
    setInterval(loadPhysicalPorts, 3000);
    
    render();
});

console.log('main.js loaded');