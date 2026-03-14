// burnscope frontend

let state = {
    isRunning: false,
    lowerType: 'virtual',
    upperPort: '',
    lowerPort: ''
};

// Initialize - 延迟调用确保 Wails 绑定准备好
window.addEventListener('load', async () => {
    console.log('window.load event');
    
    // 等待 Wails 绑定准备好
    await new Promise(resolve => setTimeout(resolve, 100));
    
    // 监听事件
    if (window.runtime && window.runtime.EventsOn) {
        window.runtime.EventsOn('data', onData);
        window.runtime.EventsOn('compare', onCompare);
        window.runtime.EventsOn('replay', onReplay);
        window.runtime.EventsOn('stats', onStats);
        window.runtime.EventsOn('error', onError);
        console.log('Events registered');
    } else {
        console.error('window.runtime not available');
    }
    
    // 初始化端口
    await initPorts();
    
    // 加载物理串口列表
    await loadPhysicalPorts();
    setInterval(loadPhysicalPorts, 3000);
});

// 初始化端口
async function initPorts() {
    console.log('initPorts: starting...');
    
    // 检查 Wails 绑定
    if (!window.go || !window.go.main || !window.go.main.App) {
        console.error('Wails bindings not ready');
        document.getElementById('upper-port').textContent = 'Wails 未就绪';
        document.getElementById('upper-port').style.color = '#f85149';
        document.getElementById('lower-port').textContent = 'Wails 未就绪';
        document.getElementById('lower-port').style.color = '#f85149';
        return;
    }
    
    try {
        console.log('Calling App.InitPorts...');
        const ports = await window.go.main.App.InitPorts();
        console.log('InitPorts returned:', ports, typeof ports);
        
        if (ports && ports.upper && ports.lower) {
            state.upperPort = ports.upper;
            state.lowerPort = ports.lower;
            updatePorts();
            console.log('Ports set: upper=' + ports.upper + ', lower=' + ports.lower);
        } else if (ports === null) {
            console.error('InitPorts returned null');
            document.getElementById('upper-port').textContent = '返回 null';
            document.getElementById('upper-port').style.color = '#f85149';
            document.getElementById('lower-port').textContent = 'PTY 创建失败?';
            document.getElementById('lower-port').style.color = '#f85149';
        } else {
            console.error('InitPorts returned invalid:', ports);
            document.getElementById('upper-port').textContent = '无效: ' + JSON.stringify(ports);
            document.getElementById('upper-port').style.color = '#f85149';
            document.getElementById('lower-port').textContent = '无效结果';
            document.getElementById('lower-port').style.color = '#f85149';
        }
    } catch (e) {
        console.error('InitPorts error:', e);
        document.getElementById('upper-port').textContent = '错误: ' + String(e);
        document.getElementById('upper-port').style.color = '#f85149';
        document.getElementById('lower-port').textContent = '错误';
        document.getElementById('lower-port').style.color = '#f85149';
    }
}

// 更新端口显示
function updatePorts() {
    const upperEl = document.getElementById('upper-port');
    const lowerEl = document.getElementById('lower-port');
    if (upperEl) upperEl.textContent = state.upperPort || '空';
    if (lowerEl) lowerEl.textContent = state.lowerPort || '空';
}

// 加载物理串口
async function loadPhysicalPorts() {
    try {
        if (window.go && window.go.main && window.go.main.App) {
            const ports = await window.go.main.App.ListSerialPorts();
            const select = document.getElementById('physical-port');
            if (select) {
                select.innerHTML = '';
                if (ports.length === 0) {
                    select.innerHTML = '<option value="">无物理串口</option>';
                } else {
                    ports.forEach(port => {
                        const opt = document.createElement('option');
                        opt.value = port;
                        opt.textContent = port;
                        select.appendChild(opt);
                    });
                }
            }
        }
    } catch (e) {
        console.error('Load ports failed:', e);
    }
}

// 下位类型变化
function onLowerTypeChange() {
    state.lowerType = document.getElementById('lower-type').value;
    
    const virtualEl = document.getElementById('virtual-config');
    const physicalEl = document.getElementById('physical-config');
    const compareEl = document.getElementById('compare-config');
    
    if (virtualEl) virtualEl.style.display = state.lowerType === 'virtual' ? 'block' : 'none';
    if (physicalEl) physicalEl.style.display = state.lowerType === 'physical' ? 'block' : 'none';
    if (compareEl) compareEl.style.display = state.lowerType === 'compare' ? 'block' : 'none';
    
    const btn = document.getElementById('action-btn');
    if (btn) {
        switch (state.lowerType) {
            case 'virtual': btn.textContent = '开始模拟'; break;
            case 'physical': btn.textContent = '开始录制'; break;
            case 'compare': btn.textContent = '开始对比'; break;
        }
    }
}

// 开始/停止
async function toggleAction() {
    if (state.isRunning) {
        await stop();
    } else {
        await start();
    }
}

// 开始
async function start() {
    const btn = document.getElementById('action-btn');
    
    try {
        switch (state.lowerType) {
            case 'virtual':
                await window.go.main.App.StartSimulate();
                break;
            case 'physical':
                const port = document.getElementById('physical-port').value;
                const baud = parseInt(document.getElementById('baud-rate').value);
                if (!port) {
                    alert('请选择物理串口');
                    return;
                }
                await window.go.main.App.StartRecord(port, baud);
                break;
            case 'compare':
                await window.go.main.App.StartCompare();
                break;
        }
        
        state.isRunning = true;
        btn.textContent = '停止';
        btn.classList.add('stop');
        document.getElementById('status-dot').classList.add('running');
        document.getElementById('status-text').textContent = '运行中';
        clearRecords();
        
    } catch (e) {
        alert('���动失败: ' + e);
    }
}

// 停止
async function stop() {
    try {
        await window.go.main.App.Stop();
        
        state.isRunning = false;
        const btn = document.getElementById('action-btn');
        btn.classList.remove('stop');
        onLowerTypeChange();
        
        document.getElementById('status-dot').classList.remove('running');
        document.getElementById('status-text').textContent = '就绪';
        
    } catch (e) {
        console.error('Stop failed:', e);
    }
}

// 保存录制
async function saveSession() {
    const file = document.getElementById('golden-file').value || 'session.golden';
    try {
        await window.go.main.App.SaveSession(file);
        alert('已保存: ' + file);
    } catch (e) {
        alert('保存失败: ' + e);
    }
}

// 加载基准
async function loadGolden() {
    const file = document.getElementById('golden-file').value || 'session.golden';
    try {
        await window.go.main.App.LoadSession(file);
        document.getElementById('golden-info').textContent = '已加载';
    } catch (e) {
        alert('加载失败: ' + e);
    }
}

// 清空记录
function clearRecords() {
    const records = document.getElementById('records');
    if (records) {
        records.innerHTML = '<div class="empty-state" id="empty-state"><p>等待数据...</p></div>';
    }
    const tx = document.getElementById('stat-tx');
    const rx = document.getElementById('stat-rx');
    const match = document.getElementById('stat-match');
    const diff = document.getElementById('stat-diff');
    if (tx) tx.textContent = '0';
    if (rx) rx.textContent = '0';
    if (match) match.textContent = '0';
    if (diff) diff.textContent = '0';
}

// 数据事件
function onData(data) {
    const records = document.getElementById('records');
    const empty = document.getElementById('empty-state');
    if (empty) empty.style.display = 'none';
    
    const row = document.createElement('div');
    row.className = 'record-row';
    
    const dirClass = data.direction.toLowerCase();
    row.innerHTML = '<span class="dir ' + dirClass + '">' + data.direction + '</span>' +
        '<span class="size">' + data.size + 'B</span>' +
        '<span class="data">' + truncateHex(data.data, 48) + '</span>';
    
    records.appendChild(row);
    records.scrollTop = records.scrollHeight;
}

// 对比事件
function onCompare(data) {
    const records = document.getElementById('records');
    const empty = document.getElementById('empty-state');
    if (empty) empty.style.display = 'none';
    
    if (data.expected) {
        const expectedRow = document.createElement('div');
        expectedRow.className = 'record-row';
        expectedRow.innerHTML = '<span class="dir tx">TX</span>' +
            '<span class="size">' + (data.expected.data.length / 2) + 'B</span>' +
            '<span class="data">基准: ' + truncateHex(data.expected.data, 40) + '</span>';
        records.appendChild(expectedRow);
    }
    
    const actualRow = document.createElement('div');
    actualRow.className = 'record-row';
    const matchClass = data.match ? 'match' : 'diff';
    const matchIcon = data.match ? '✓' : '✗';
    actualRow.innerHTML = '<span class="dir tx">TX</span>' +
        '<span class="size">' + (data.actual.data.length / 2) + 'B</span>' +
        '<span class="data">对比: ' + truncateHex(data.actual.data, 40) + '</span>' +
        '<span class="result ' + matchClass + '">' + matchIcon + '</span>';
    records.appendChild(actualRow);
    
    records.scrollTop = records.scrollHeight;
}

// 回放事件
function onReplay(data) {
    const records = document.getElementById('records');
    const row = document.createElement('div');
    row.className = 'record-row replay';
    row.innerHTML = '<span class="dir rx">RX</span>' +
        '<span class="size">' + (data.data.length / 2) + 'B</span>' +
        '<span class="data">' + truncateHex(data.data, 40) + ' (回放)</span>';
    records.appendChild(row);
    records.scrollTop = records.scrollHeight;
}

// 统计事件
function onStats(data) {
    const match = document.getElementById('stat-match');
    const diff = document.getElementById('stat-diff');
    if (match) match.textContent = data.matched;
    if (diff) diff.textContent = data.diff;
}

// 错误事件
function onError(msg) {
    console.error('Error event:', msg);
    alert('错误: ' + msg);
}

// 复制端口
function copyPort(type) {
    const port = type === 'upper' ? state.upperPort : state.lowerPort;
    if (port && navigator.clipboard) {
        navigator.clipboard.writeText(port).then(function() {
            const el = document.getElementById(type + '-port');
            if (el) {
                el.style.color = '#238636';
                setTimeout(function() { el.style.color = ''; }, 200);
            }
        });
    }
}

// 截断 hex
function truncateHex(hex, maxLen) {
    if (!hex || hex.length <= maxLen) return hex || '';
    return hex.substring(0, maxLen) + '...';
}

// 暴露全局函数
window.onLowerTypeChange = onLowerTypeChange;
window.toggleAction = toggleAction;
window.saveSession = saveSession;
window.loadGolden = loadGolden;
window.copyPort = copyPort;

console.log('main.js loaded');