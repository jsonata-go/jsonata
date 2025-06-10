// Global state
let jsonEditor, jsonataEditor, bindingsEditor;
let currentVersion = '';
let evaluationTimer = null;
let samples = {};

// Initialize the application
document.addEventListener('DOMContentLoaded', async () => {
    // Initialize split panes
    Split(['#left-pane', '#right-pane'], {
        sizes: [50, 50],
        minSize: 100,
        gutterSize: 10,
    });

    Split(['#json-pane', '#bindings-pane'], {
        sizes: [80, 20],
        minSize: 30,
        gutterSize: 10,
        direction: 'vertical',
    });

    Split(['#jsonata-pane', '#result-pane'], {
        sizes: [30, 70],
        minSize: 50,
        gutterSize: 10,
        direction: 'vertical',
    });

    // Initialize CodeMirror editors
    initializeEditors();

    // Load versions
    await loadVersions();

    // Load samples
    await loadSamples();

    // Set up event handlers
    setupEventHandlers();

    // Load initial sample
    loadSample('Event');
});

// Initialize CodeMirror editors
function initializeEditors() {
    const editorOptions = {
        lineNumbers: false,
        theme: 'default',
        mode: 'javascript',
        lineWrapping: true,
        viewportMargin: Infinity,
    };

    // JSON editor
    jsonEditor = CodeMirror.fromTextArea(document.getElementById('json-editor'), {
        ...editorOptions,
        mode: {name: 'javascript', json: true},
    });

    // JSONata editor
    jsonataEditor = CodeMirror.fromTextArea(document.getElementById('jsonata-editor'), {
        ...editorOptions,
        mode: 'javascript', // We'll use JavaScript mode for now
    });

    // Bindings editor
    bindingsEditor = CodeMirror.fromTextArea(document.getElementById('bindings-editor'), {
        ...editorOptions,
        mode: 'javascript',
    });

    // Set up change handlers
    jsonEditor.on('change', debounceEvaluate);
    jsonataEditor.on('change', debounceEvaluate);
    bindingsEditor.on('change', debounceEvaluate);
}

// Load available versions
async function loadVersions() {
    try {
        const response = await fetch('/api/versions');
        const data = await response.json();
        
        const select = document.getElementById('version-select');
        select.innerHTML = '';
        
        data.versions.forEach((version, index) => {
            const option = document.createElement('option');
            option.value = version;
            option.textContent = version;
            if (index === 0) {
                currentVersion = version;
                option.selected = true;
            }
            select.appendChild(option);
        });
    } catch (error) {
        console.error('Failed to load versions:', error);
    }
}

// Load samples
async function loadSamples() {
    try {
        const response = await fetch('/api/samples');
        const data = await response.json();
        
        data.forEach(sample => {
            samples[sample.name] = sample;
        });
    } catch (error) {
        console.error('Failed to load samples:', error);
    }
}

// Setup event handlers
function setupEventHandlers() {
    // Version change
    document.getElementById('version-select').addEventListener('change', (e) => {
        currentVersion = e.target.value;
        evaluate();
    });

    // Sample change
    document.getElementById('sample-select').addEventListener('change', (e) => {
        loadSample(e.target.value);
    });

    // Format JSON button
    document.getElementById('format-json').addEventListener('click', formatJSON);

}

// Load a sample
function loadSample(sampleName) {
    const sample = samples[sampleName];
    if (!sample) return;

    // Update editors
    jsonEditor.setValue(JSON.stringify(sample.json, null, 2));
    jsonataEditor.setValue(sample.jsonata);
    bindingsEditor.setValue(sample.bindings);

    // Clear any error states
    clearErrors();

    // Evaluate immediately
    evaluate();
}

// Format JSON
function formatJSON() {
    try {
        const json = JSON.parse(jsonEditor.getValue());
        jsonEditor.setValue(JSON.stringify(json, null, 2));
    } catch (error) {
        // Silently fail if JSON is invalid
    }
}


// Debounce evaluation
function debounceEvaluate() {
    clearTimeout(evaluationTimer);
    evaluationTimer = setTimeout(evaluate, 500);
}

// Evaluate expression
async function evaluate() {
    const resultContent = document.getElementById('result-content');
    
    try {
        // Get values from editors
        const jsonValue = jsonEditor.getValue();
        const expression = jsonataEditor.getValue();
        const bindings = bindingsEditor.getValue();

        // Parse JSON input
        let input;
        try {
            input = jsonValue ? JSON.parse(jsonValue) : {};
        } catch (error) {
            showError('JSON Error: ' + error.message);
            return;
        }

        // Clear previous errors
        clearErrors();

        // Make evaluation request
        const response = await fetch('/api/evaluate', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                expression: expression,
                input: input,
                bindings: bindings,
                version: currentVersion,
            }),
        });

        const data = await response.json();

        if (data.error) {
            showError(data.error);
        } else {
            // Format result
            let resultText;
            if (data.result === undefined || data.result === null) {
                resultText = '** no match **';
            } else if (typeof data.result === 'string') {
                resultText = data.result;
            } else {
                resultText = JSON.stringify(data.result, null, 2);
            }
            
            resultContent.textContent = resultText;
            resultContent.classList.remove('result-error');
        }
    } catch (error) {
        showError('Network error: ' + error.message);
    }
}

// Show error
function showError(message) {
    const resultContent = document.getElementById('result-content');
    resultContent.textContent = message;
    resultContent.classList.add('result-error');
}

// Clear errors
function clearErrors() {
    const resultContent = document.getElementById('result-content');
    resultContent.classList.remove('result-error');
}
