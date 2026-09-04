(function () {
  "use strict";

  var adminConfig = window.__WEBHOOK_ADMIN__ || {};
  var basePath = adminConfig.basePath || "";

  var sourceOptions = [
    { value: "", label: "Select source / 选择来源" },
    { value: "header", label: "Header / 请求头" },
    { value: "url", label: "URL Query / URL查询参数" },
    { value: "query", label: "Query Alias / 查询别名" },
    { value: "payload", label: "Payload / 请求体" },
    { value: "raw-request-body", label: "Raw Request Body / 原始请求体" },
    { value: "request", label: "Request / 请求信息" },
    { value: "string", label: "Static String / 静态字符串" },
    { value: "entire-payload", label: "Entire Payload / 完整请求体" },
    { value: "entire-query", label: "Entire Query / 完整查询参数" },
    { value: "entire-headers", label: "Entire Headers / 完整请求头" }
  ];

  var matchTypeOptions = [
    { value: "value", label: "Value Equals / 值匹配" },
    { value: "regex", label: "Regex Match / 正则匹配" },
    { value: "payload-hmac-sha1", label: "Payload HMAC SHA1 / 签名验证" },
    { value: "payload-hmac-sha256", label: "Payload HMAC SHA256 / 签名验证" },
    { value: "payload-hmac-sha512", label: "Payload HMAC SHA512 / 签名验证" },
    { value: "payload-hash-sha1", label: "Payload Hash SHA1 (已弃用)" },
    { value: "payload-hash-sha256", label: "Payload Hash SHA256 (已弃用)" },
    { value: "payload-hash-sha512", label: "Payload Hash SHA512 (已弃用)" },
    { value: "ip-whitelist", label: "IP Whitelist / IP白名单" },
    { value: "scalr-signature", label: "Scalr Signature / Scalr签名" }
  ];

  var state = {
    files: [],
    currentFile: "",
    currentHookId: "",
    readOnly: false,
    readOnlyReason: "",
    ruleDraft: null
  };

  function byId(id) {
    return document.getElementById(id);
  }

  var els = {
    loginPanel: byId("loginPanel"),
    workspace: byId("workspace"),
    loginForm: byId("loginForm"),
    totpCode: byId("totpCode"),
    loginStatus: byId("loginStatus"),
    fileSelect: byId("fileSelect"),
    fileMeta: byId("fileMeta"),
    hookList: byId("hookList"),
    refreshBtn: byId("refreshBtn"),
    checkUpdateBtn: byId("checkUpdateBtn"),
    currentVersion: byId("currentVersion"),
    latestVersion: byId("latestVersion"),
    updateStatus: byId("updateStatus"),
    newHookBtn: byId("newHookBtn"),
    logoutBtn: byId("logoutBtn"),
    readOnlyBanner: byId("readOnlyBanner"),
    editorFieldset: byId("editorFieldset"),
    hookForm: byId("hookForm"),
    hookId: byId("hookId"),
    executeCommand: byId("executeCommand"),
    commandWorkingDirectory: byId("commandWorkingDirectory"),
    commandTimeout: byId("commandTimeout"),
    httpMethods: byId("httpMethods"),
    responseMessage: byId("responseMessage"),
    incomingPayloadContentType: byId("incomingPayloadContentType"),
    successHttpResponseCode: byId("successHttpResponseCode"),
    triggerRuleMismatchHttpResponseCode: byId("triggerRuleMismatchHttpResponseCode"),
    maxConcurrency: byId("maxConcurrency"),
    captureCommandOutput: byId("captureCommandOutput"),
    captureCommandOutputOnError: byId("captureCommandOutputOnError"),
    triggerSignatureSoftFailures: byId("triggerSignatureSoftFailures"),
    keepFileEnvironment: byId("keepFileEnvironment"),
    responseHeadersRows: byId("responseHeadersRows"),
    passArgumentsRows: byId("passArgumentsRows"),
    passEnvironmentRows: byId("passEnvironmentRows"),
    passFileRows: byId("passFileRows"),
    parseJSONRows: byId("parseJSONRows"),
    addResponseHeaderBtn: byId("addResponseHeaderBtn"),
    addPassArgumentBtn: byId("addPassArgumentBtn"),
    addPassEnvironmentBtn: byId("addPassEnvironmentBtn"),
    addPassFileBtn: byId("addPassFileBtn"),
    addParseJSONBtn: byId("addParseJSONBtn"),
    ruleEditor: byId("ruleEditor"),
    saveBtn: byId("saveBtn"),
    deleteBtn: byId("deleteBtn"),
    jsonPreview: byId("jsonPreview"),
    workspaceStatus: byId("workspaceStatus")
  };

  var collectionDefs = {
    responseHeaders: {
      jsonKey: "response-headers",
      title: "response headers",
      emptyText: "暂无响应头。",
      container: els.responseHeadersRows,
      addButton: els.addResponseHeaderBtn,
      fields: [
        { name: "name", label: "Name", type: "text", required: true },
        { name: "value", label: "Value", type: "text" }
      ]
    },
    passArguments: {
      jsonKey: "pass-arguments-to-command",
      title: "command arguments",
      emptyText: "暂无命令参数。",
      container: els.passArgumentsRows,
      addButton: els.addPassArgumentBtn,
      fields: [
        { name: "source", label: "Source", type: "select", options: sourceOptions, required: true },
        { name: "name", label: "Name", type: "text", required: true }
      ]
    },
    passEnvironment: {
      jsonKey: "pass-environment-to-command",
      title: "environment mappings",
      emptyText: "暂无环境变量映射。",
      container: els.passEnvironmentRows,
      addButton: els.addPassEnvironmentBtn,
      fields: [
        { name: "source", label: "Source", type: "select", options: sourceOptions, required: true },
        { name: "name", label: "Name", type: "text", required: true },
        { name: "envname", label: "Env Name", type: "text" }
      ]
    },
    passFile: {
      jsonKey: "pass-file-to-command",
      title: "file mappings",
      emptyText: "暂无文件参数。",
      container: els.passFileRows,
      addButton: els.addPassFileBtn,
      fields: [
        { name: "source", label: "Source", type: "select", options: sourceOptions, required: true },
        { name: "name", label: "Name", type: "text", required: true },
        { name: "envname", label: "Env Name", type: "text" },
        { name: "base64decode", label: "Base64", type: "checkbox" }
      ]
    },
    parseJSON: {
      jsonKey: "parse-parameters-as-json",
      title: "JSON parameter mappings",
      emptyText: "暂无 JSON 参数映射。",
      container: els.parseJSONRows,
      addButton: els.addParseJSONBtn,
      fields: [
        { name: "source", label: "Source", type: "select", options: sourceOptions, required: true },
        { name: "name", label: "Name", type: "text", required: true }
      ]
    }
  };

  function clone(value) {
    if (value === null || value === undefined) {
      return value;
    }

    return JSON.parse(JSON.stringify(value));
  }

  function normalizeArray(value) {
    return Array.isArray(value) ? value : [];
  }

  function normalizeHookFile(file) {
    var next = clone(file) || {};
    next.hooks = normalizeArray(next.hooks);
    return next;
  }

  function normalizeHook(hook) {
    var next = clone(hook) || {};
    next["response-headers"] = normalizeArray(next["response-headers"]);
    next["pass-arguments-to-command"] = normalizeArray(next["pass-arguments-to-command"]);
    next["pass-environment-to-command"] = normalizeArray(next["pass-environment-to-command"]);
    next["pass-file-to-command"] = normalizeArray(next["pass-file-to-command"]);
    next["parse-parameters-as-json"] = normalizeArray(next["parse-parameters-as-json"]);
    next["http-methods"] = normalizeArray(next["http-methods"]);
    return next;
  }

  function emptyHook() {
    return normalizeHook({
      id: "",
      "execute-command": "",
      "command-working-directory": "",
      "command-timeout": "",
      "response-message": "",
      "http-methods": ["POST"]
    });
  }

  function splitCSV(value) {
    return String(value || "")
      .split(",")
      .map(function (item) {
        return item.trim().toUpperCase();
      })
      .filter(function (item) {
        return item !== "";
      });
  }

  function setStatus(target, message, tone) {
    target.textContent = message;
    target.className = "status " + (tone ? "status-" + tone : "status-muted");
  }

  function showLogin(message) {
    els.workspace.hidden = true;
    els.loginPanel.hidden = false;
    setStatus(els.loginStatus, message || "请输入 TOTP 动态码。", message ? "error" : "muted");
  }

  function showWorkspace() {
    els.loginPanel.hidden = true;
    els.workspace.hidden = false;
  }

  function request(path, options) {
    var fetchOptions = options || {};
    fetchOptions.credentials = "same-origin";
    fetchOptions.headers = fetchOptions.headers || {};
    fetchOptions.headers.Accept = "application/json";

    return fetch(basePath + path, fetchOptions).then(function (response) {
      return response.text().then(function (text) {
        var data = null;
        if (text) {
          try {
            data = JSON.parse(text);
          } catch (error) {
            data = null;
          }
        }

        if (response.status === 401) {
          showLogin("会话已失效，请重新登录。");
          throw new Error("unauthorized");
        }

        if (!response.ok) {
          throw new Error(data && data.error ? data.error : "请求失败。");
        }

        return data;
      });
    });
  }

  function getCurrentFile() {
    for (var i = 0; i < state.files.length; i++) {
      if (state.files[i].path === state.currentFile) {
        return state.files[i];
      }
    }

    return null;
  }

  function getCurrentHook() {
    var currentFile = getCurrentFile();
    var hooks = currentFile ? normalizeArray(currentFile.hooks) : [];

    for (var i = 0; i < hooks.length; i++) {
      if (hooks[i].id === state.currentHookId) {
        return hooks[i];
      }
    }

    return null;
  }

  function createOption(option, selectedValue) {
    var el = document.createElement("option");
    el.value = option.value;
    el.textContent = option.label;
    if (option.value === selectedValue) {
      el.selected = true;
    }
    return el;
  }

  function inputValue(el) {
    return String(el.value || "").trim();
  }

  function appendEmptyState(container, text) {
    var empty = document.createElement("div");
    empty.className = "repeatable-empty";
    empty.textContent = text;
    container.appendChild(empty);
  }

  function clearEmptyState(container) {
    var empty = container.querySelector(".repeatable-empty");
    if (empty) {
      empty.remove();
    }
  }

  function ensureCollectionState(def) {
    if (!def.container.querySelector(".repeatable-row")) {
      def.container.innerHTML = "";
      appendEmptyState(def.container, def.emptyText);
    }
  }

  function createRowField(field, value) {
    var wrapper = document.createElement("div");

    if (field.type === "checkbox") {
      wrapper.className = "row-check";

      var checkbox = document.createElement("input");
      checkbox.type = "checkbox";
      checkbox.checked = !!value;
      checkbox.dataset.field = field.name;
      checkbox.title = field.label;
      checkbox.setAttribute("aria-label", field.label);
      checkbox.addEventListener("change", updatePreview);
      wrapper.appendChild(checkbox);
      return wrapper;
    }

    var label = document.createElement("span");
    label.className = "field-inline-label";
    label.textContent = field.label;
    wrapper.appendChild(label);

    var input;
    if (field.type === "select") {
      input = document.createElement("select");
      field.options.forEach(function (option) {
        input.appendChild(createOption(option, value || ""));
      });
    } else {
      input = document.createElement("input");
      input.type = "text";
      input.value = value || "";
    }

    input.dataset.field = field.name;
    input.addEventListener("input", updatePreview);
    input.addEventListener("change", updatePreview);
    wrapper.appendChild(input);

    return wrapper;
  }

  function appendCollectionRow(def, rowData) {
    clearEmptyState(def.container);

    var row = document.createElement("div");
    row.className = "repeatable-row columns-" + def.fields.length;

    def.fields.forEach(function (field) {
      row.appendChild(createRowField(field, rowData ? rowData[field.name] : ""));
    });

    var remove = document.createElement("button");
    remove.type = "button";
    remove.className = "icon-button";
    remove.textContent = "×";
    remove.title = "删除这一行";
    remove.addEventListener("click", function () {
      row.remove();
      ensureCollectionState(def);
      updatePreview();
    });
    row.appendChild(remove);

    def.container.appendChild(row);
  }

  function renderCollection(def, rows) {
    def.container.innerHTML = "";

    var safeRows = normalizeArray(rows);
    if (!safeRows.length) {
      appendEmptyState(def.container, def.emptyText);
      return;
    }

    safeRows.forEach(function (row) {
      appendCollectionRow(def, row);
    });
  }

  function readCollection(def, errors) {
    var items = [];
    var rows = def.container.querySelectorAll(".repeatable-row");

    rows.forEach(function (row, index) {
      var item = {};
      var used = false;

      def.fields.forEach(function (field) {
        var input = row.querySelector("[data-field='" + field.name + "']");
        if (!input) {
          return;
        }

        if (field.type === "checkbox") {
          if (input.checked) {
            item[field.name] = true;
            used = true;
          }
          return;
        }

        var value = String(input.value || "").trim();
        if (value !== "") {
          item[field.name] = value;
          used = true;
        }
      });

      if (!used) {
        return;
      }

      def.fields.forEach(function (field) {
        if (!field.required || field.type === "checkbox") {
          return;
        }

        if (!item[field.name]) {
          errors.push("Incomplete " + def.title + " row " + (index + 1) + ": " + field.label + " is required.");
        }
      });

      items.push(item);
    });

    return items;
  }

  function ruleKind(rule) {
    if (!rule) {
      return "none";
    }
    if (Array.isArray(rule.and)) {
      return "and";
    }
    if (Array.isArray(rule.or)) {
      return "or";
    }
    if (rule.not) {
      return "not";
    }
    if (rule.match) {
      return "match";
    }
    return "none";
  }

  function createRule(kind) {
    switch (kind) {
      case "and":
        return { and: [createRule("match")] };
      case "or":
        return { or: [createRule("match")] };
      case "not":
        return { not: createRule("match") };
      case "match":
        return {
          match: {
            type: "value",
            parameter: {
              source: "",
              name: ""
            },
            value: ""
          }
        };
      default:
        return null;
    }
  }

  function ensureMatch(rule) {
    if (!rule.match) {
      rule.match = createRule("match").match;
    }
    if (!rule.match.parameter) {
      rule.match.parameter = { source: "", name: "" };
    }
    return rule.match;
  }

  function isSignatureRule(type) {
    return (
      type === "payload-hmac-sha1" ||
      type === "payload-hmac-sha256" ||
      type === "payload-hmac-sha512" ||
      type === "payload-hash-sha1" ||
      type === "payload-hash-sha256" ||
      type === "payload-hash-sha512"
    );
  }

  function needsParameter(type) {
    return type !== "ip-whitelist" && type !== "scalr-signature";
  }

  function createLabeledField(labelText, control) {
    var wrapper = document.createElement("div");
    var label = document.createElement("label");
    label.textContent = labelText;
    wrapper.appendChild(label);
    wrapper.appendChild(control);
    return wrapper;
  }

  function createTextInput(value, placeholder, onChange) {
    var input = document.createElement("input");
    input.type = "text";
    input.value = value || "";
    input.placeholder = placeholder || "";
    input.addEventListener("input", function () {
      onChange(input.value);
      updatePreview();
    });
    return input;
  }

  function createSelectInput(options, value, onChange) {
    var select = document.createElement("select");
    options.forEach(function (option) {
      select.appendChild(createOption(option, value));
    });
    select.addEventListener("change", function () {
      onChange(select.value);
    });
    return select;
  }

  function renderMatchRule(card, rule) {
    var match = ensureMatch(rule);
    var fields = document.createElement("div");
    fields.className = "grid grid-2";

    var typeSelect = createSelectInput(matchTypeOptions, match.type || "value", function (nextType) {
      match.type = nextType;
      renderRuleEditor();
      updatePreview();
    });
    fields.appendChild(createLabeledField("Match Type", typeSelect));

    if (needsParameter(match.type)) {
      var sourceSelect = createSelectInput(sourceOptions, match.parameter ? match.parameter.source : "", function (nextSource) {
        ensureMatch(rule).parameter.source = nextSource;
        updatePreview();
      });
      fields.appendChild(createLabeledField("Parameter Source", sourceSelect));

      var nameInput = createTextInput(match.parameter ? match.parameter.name : "", "e.g. X-Hub-Signature-256", function (nextName) {
        ensureMatch(rule).parameter.name = nextName;
      });
      fields.appendChild(createLabeledField("Parameter Name", nameInput));
    }

    if (match.type === "value") {
      fields.appendChild(createLabeledField("Expected Value", createTextInput(match.value, "main", function (nextValue) {
        ensureMatch(rule).value = nextValue;
      })));
    }

    if (match.type === "regex") {
      fields.appendChild(createLabeledField("Regex", createTextInput(match.regex, "^refs/heads/main$", function (nextRegex) {
        ensureMatch(rule).regex = nextRegex;
      })));
    }

    if (isSignatureRule(match.type) || match.type === "scalr-signature") {
      fields.appendChild(createLabeledField("Secret", createTextInput(match.secret, "shared secret", function (nextSecret) {
        ensureMatch(rule).secret = nextSecret;
      })));
    }

    if (match.type === "ip-whitelist") {
      fields.appendChild(createLabeledField("IP Range (IP白名单)", createTextInput(match["ip-range"] || match.ipRange, "192.168.1.0/24 10.0.0.1", function (nextRange) {
        ensureMatch(rule)["ip-range"] = nextRange;
      })));
      var hint = document.createElement("div");
      hint.className = "field-hint";
      hint.textContent = "支持 CIDR 和单 IP，空格分隔多个。若经过反向代理，需配置 --trusted-proxies 和 --real-ip-header 才能获取真实客户端 IP。";
      fields.appendChild(hint);
    }

    card.appendChild(fields);
  }

  function renderRuleCard(rule, depth, replaceRule, removeRule, isRoot) {
    var card = document.createElement("div");
    card.className = "rule-card";
    card.dataset.depth = String(Math.min(depth, 3));

    var head = document.createElement("div");
    head.className = "rule-head";

    var title = document.createElement("strong");
    title.textContent = isRoot ? "Root Rule" : "Rule";
    head.appendChild(title);

    var controls = document.createElement("div");
    controls.className = "toolbar-row";

    var kindSelect = createSelectInput(
      [
        { value: "none", label: "No Rule" },
        { value: "match", label: "Match" },
        { value: "and", label: "And" },
        { value: "or", label: "Or" },
        { value: "not", label: "Not" }
      ],
      ruleKind(rule),
      function (nextKind) {
        replaceRule(createRule(nextKind));
        renderRuleEditor();
        updatePreview();
      }
    );
    controls.appendChild(kindSelect);

    if (removeRule) {
      var remove = document.createElement("button");
      remove.type = "button";
      remove.className = "icon-button";
      remove.textContent = "×";
      remove.title = "删除这一条规则";
      remove.addEventListener("click", function () {
        removeRule();
        renderRuleEditor();
        updatePreview();
      });
      controls.appendChild(remove);
    }

    head.appendChild(controls);
    card.appendChild(head);

    var kind = ruleKind(rule);
    if (kind === "none") {
      var note = document.createElement("div");
      note.className = "rule-note";
      note.textContent = "当前没有启用触发规则。";
      card.appendChild(note);
      return card;
    }

    if (kind === "match") {
      renderMatchRule(card, rule);
      return card;
    }

    if (kind === "not") {
      if (!rule.not) {
        rule.not = createRule("match");
      }

      var single = document.createElement("div");
      single.className = "rule-children";
      single.appendChild(
        renderRuleCard(
          rule.not,
          depth + 1,
          function (nextRule) {
            rule.not = nextRule;
          },
          null,
          false
        )
      );
      card.appendChild(single);
      return card;
    }

    var listKey = kind;
    if (!Array.isArray(rule[listKey])) {
      rule[listKey] = [];
    }

    var children = document.createElement("div");
    children.className = "rule-children";

    if (!rule[listKey].length) {
      var empty = document.createElement("div");
      empty.className = "rule-note";
      empty.textContent = "当前没有子规则。";
      children.appendChild(empty);
    } else {
      rule[listKey].forEach(function (childRule, index) {
        children.appendChild(
          renderRuleCard(
            childRule,
            depth + 1,
            function (nextRule) {
              rule[listKey][index] = nextRule;
            },
            function () {
              rule[listKey].splice(index, 1);
            },
            false
          )
        );
      });
    }

    card.appendChild(children);

    var addChild = document.createElement("button");
    addChild.type = "button";
    addChild.className = "ghost";
    addChild.textContent = "添加子规则";
    addChild.addEventListener("click", function () {
      rule[listKey].push(createRule("match"));
      renderRuleEditor();
      updatePreview();
    });
    card.appendChild(addChild);

    return card;
  }

  function renderRuleEditor() {
    els.ruleEditor.innerHTML = "";
    els.ruleEditor.appendChild(
      renderRuleCard(
        state.ruleDraft,
        0,
        function (nextRule) {
          state.ruleDraft = nextRule;
        },
        null,
        true
      )
    );
  }

  function setFormValue(el, value) {
    el.value = value || "";
  }

  function renderCurrentHook() {
    var hook = normalizeHook(getCurrentHook() || emptyHook());

    setFormValue(els.hookId, hook.id);
    setFormValue(els.executeCommand, hook["execute-command"]);
    setFormValue(els.commandWorkingDirectory, hook["command-working-directory"]);
    setFormValue(els.commandTimeout, hook["command-timeout"]);
    setFormValue(els.httpMethods, normalizeArray(hook["http-methods"]).join(", "));
    setFormValue(els.responseMessage, hook["response-message"]);
    setFormValue(els.incomingPayloadContentType, hook["incoming-payload-content-type"]);
    setFormValue(els.successHttpResponseCode, hook["success-http-response-code"] || "");
    setFormValue(els.triggerRuleMismatchHttpResponseCode, hook["trigger-rule-mismatch-http-response-code"] || "");
    setFormValue(els.maxConcurrency, hook["max-concurrency"] === 0 ? "0" : (hook["max-concurrency"] || ""));

    els.captureCommandOutput.checked = !!hook["include-command-output-in-response"];
    els.captureCommandOutputOnError.checked = !!hook["include-command-output-in-response-on-error"];
    els.triggerSignatureSoftFailures.checked = !!hook["trigger-signature-soft-failures"];
    els.keepFileEnvironment.checked = !!hook["keep-file-environment"];

    renderCollection(collectionDefs.responseHeaders, hook["response-headers"]);
    renderCollection(collectionDefs.passArguments, hook["pass-arguments-to-command"]);
    renderCollection(collectionDefs.passEnvironment, hook["pass-environment-to-command"]);
    renderCollection(collectionDefs.passFile, hook["pass-file-to-command"]);
    renderCollection(collectionDefs.parseJSON, hook["parse-parameters-as-json"]);

    state.ruleDraft = clone(hook["trigger-rule"]) || null;
    renderRuleEditor();
    updatePreview();
  }

  function renderFileOptions() {
    els.fileSelect.innerHTML = "";
    els.fileSelect.disabled = state.files.length === 0;

    if (!state.files.length) {
      var empty = document.createElement("option");
      empty.value = "";
      empty.textContent = "No hooks file";
      els.fileSelect.appendChild(empty);
      return;
    }

    state.files.forEach(function (file) {
      var option = document.createElement("option");
      option.value = file.path;
      option.textContent = file.path;
      if (file.path === state.currentFile) {
        option.selected = true;
      }
      els.fileSelect.appendChild(option);
    });
  }

  function renderFileMeta() {
    var currentFile = getCurrentFile();
    if (!currentFile) {
      els.fileMeta.textContent = "暂无可管理的 hooks 文件。";
      return;
    }

    els.fileMeta.textContent =
      "格式: " +
      String(currentFile.format || "json").toUpperCase() +
      " | hooks: " +
      normalizeArray(currentFile.hooks).length +
      " | 路径: " +
      currentFile.path;
  }

  function renderHookList() {
    els.hookList.innerHTML = "";

    var currentFile = getCurrentFile();
    var hooks = currentFile ? normalizeArray(currentFile.hooks) : [];

    if (!hooks.length) {
      appendEmptyState(els.hookList, "当前文件还没有 hook，可以直接点击“新建”。");
      return;
    }

    hooks.forEach(function (hookItem) {
      var button = document.createElement("button");
      button.type = "button";
      button.className = "hook-item" + (hookItem.id === state.currentHookId ? " active" : "");
      button.dataset.id = hookItem.id;

      var methods = normalizeArray(hookItem["http-methods"]).length ? hookItem["http-methods"].join(", ") : "ALL";
      var command = hookItem["execute-command"] || "(no execute-command)";

      var title = document.createElement("strong");
      title.textContent = hookItem.id || "(no id)";
      button.appendChild(title);

      var subtitle = document.createElement("small");
      subtitle.textContent = methods + " · " + command;
      button.appendChild(subtitle);

      button.addEventListener("click", function () {
        state.currentHookId = hookItem.id;
        renderHookList();
        renderCurrentHook();
        setStatus(els.workspaceStatus, "已载入 hook “" + hookItem.id + "”。", "muted");
      });

      els.hookList.appendChild(button);
    });
  }

  function applyReadOnlyState() {
    var hasFiles = state.files.length > 0;
    var canEdit = hasFiles && !state.readOnly;

    if (state.readOnly) {
      els.readOnlyBanner.hidden = false;
      els.readOnlyBanner.textContent = state.readOnlyReason || "当前运行模式为只读。";
    } else {
      els.readOnlyBanner.hidden = true;
      els.readOnlyBanner.textContent = "";
    }

    els.editorFieldset.disabled = !canEdit;
    els.newHookBtn.disabled = !canEdit;
    els.saveBtn.disabled = !canEdit;
    els.deleteBtn.disabled = !canEdit || !state.currentHookId;
  }

  function renderAll() {
    renderFileOptions();
    renderFileMeta();
    renderHookList();
    renderCurrentHook();
    applyReadOnlyState();
  }

  function parseOptionalInt(value, label, errors) {
    var trimmed = String(value || "").trim();
    if (trimmed === "") {
      return null;
    }

    var parsed = parseInt(trimmed, 10);
    if (isNaN(parsed)) {
      errors.push(label + " must be a valid integer.");
      return null;
    }

    return parsed;
  }

  function cleanRule(rule, errors, pathLabel) {
    var kind = ruleKind(rule);
    if (kind === "none") {
      return null;
    }

    if (kind === "match") {
      var match = clone((rule && rule.match) || {});
      var type = match.type || "value";
      var cleanedMatch = { type: type };

      if (needsParameter(type)) {
        var parameter = clone(match.parameter) || {};
        parameter.source = String(parameter.source || "").trim();
        parameter.name = String(parameter.name || "").trim();
        if (!parameter.source || !parameter.name) {
          errors.push(pathLabel + " requires parameter source and parameter name.");
        } else {
          cleanedMatch.parameter = parameter;
        }
      }

      if (type === "value") {
        var expectedValue = String(match.value || "").trim();
        if (!expectedValue) {
          errors.push(pathLabel + " requires a comparison value.");
        } else {
          cleanedMatch.value = expectedValue;
        }
      } else if (type === "regex") {
        var regex = String(match.regex || "").trim();
        if (!regex) {
          errors.push(pathLabel + " requires a regex.");
        } else {
          cleanedMatch.regex = regex;
        }
      } else if (isSignatureRule(type) || type === "scalr-signature") {
        var secret = String(match.secret || "").trim();
        if (!secret) {
          errors.push(pathLabel + " requires a secret.");
        } else {
          cleanedMatch.secret = secret;
        }
      } else if (type === "ip-whitelist") {
        var ipRange = String(match["ip-range"] || match.ipRange || "").trim();
        if (!ipRange) {
          errors.push(pathLabel + " requires an IP range.");
        } else {
          cleanedMatch["ip-range"] = ipRange;
        }
      }

      return { match: cleanedMatch };
    }

    if (kind === "not") {
      var childRule = cleanRule(rule.not, errors, pathLabel + " > not");
      if (!childRule) {
        errors.push(pathLabel + " requires a nested rule.");
        return null;
      }
      return { not: childRule };
    }

    var key = kind;
    var children = normalizeArray(rule[key]).map(function (child, index) {
      return cleanRule(child, errors, pathLabel + " > child " + (index + 1));
    }).filter(function (child) {
      return !!child;
    });

    if (!children.length) {
      errors.push(pathLabel + " requires at least one child rule.");
      return null;
    }

    var composite = {};
    composite[key] = children;
    return composite;
  }

  function buildHookFromForm() {
    var errors = [];
    var hook = {};

    var id = inputValue(els.hookId);
    if (!id) {
      errors.push("Hook ID is required.");
    } else {
      hook.id = id;
    }

    var executeCommand = inputValue(els.executeCommand);
    if (!executeCommand) {
      errors.push("Execute Command is required.");
    } else {
      hook["execute-command"] = executeCommand;
    }

    var workingDirectory = inputValue(els.commandWorkingDirectory);
    if (workingDirectory) {
      hook["command-working-directory"] = workingDirectory;
    }

    var commandTimeout = inputValue(els.commandTimeout);
    if (commandTimeout) {
      hook["command-timeout"] = commandTimeout;
    }

    var methods = splitCSV(els.httpMethods.value);
    if (methods.length) {
      hook["http-methods"] = methods;
    }

    var responseMessage = inputValue(els.responseMessage);
    if (responseMessage) {
      hook["response-message"] = responseMessage;
    }

    var contentType = inputValue(els.incomingPayloadContentType);
    if (contentType) {
      hook["incoming-payload-content-type"] = contentType;
    }

    var successCode = parseOptionalInt(els.successHttpResponseCode.value, "Success HTTP Code", errors);
    if (successCode !== null) {
      hook["success-http-response-code"] = successCode;
    }

    var mismatchCode = parseOptionalInt(els.triggerRuleMismatchHttpResponseCode.value, "Rule Mismatch HTTP Code", errors);
    if (mismatchCode !== null) {
      hook["trigger-rule-mismatch-http-response-code"] = mismatchCode;
    }

    var maxConcurrency = parseOptionalInt(els.maxConcurrency.value, "Max Concurrency", errors);
    if (maxConcurrency !== null) {
      hook["max-concurrency"] = maxConcurrency;
    }

    if (els.captureCommandOutput.checked) {
      hook["include-command-output-in-response"] = true;
    }
    if (els.captureCommandOutputOnError.checked) {
      hook["include-command-output-in-response-on-error"] = true;
    }
    if (els.triggerSignatureSoftFailures.checked) {
      hook["trigger-signature-soft-failures"] = true;
    }
    if (els.keepFileEnvironment.checked) {
      hook["keep-file-environment"] = true;
    }

    Object.keys(collectionDefs).forEach(function (key) {
      var def = collectionDefs[key];
      var items = readCollection(def, errors);
      if (items.length) {
        hook[def.jsonKey] = items;
      }
    });

    var cleanedRule = cleanRule(state.ruleDraft, errors, "Trigger rule");
    if (cleanedRule) {
      hook["trigger-rule"] = cleanedRule;
    }

    return {
      hook: hook,
      errors: errors
    };
  }

  function updatePreview() {
    var result = buildHookFromForm();
    var preview = JSON.stringify(result.hook, null, 2);

    if (result.errors.length) {
      els.jsonPreview.textContent = "// Validation issues\n// " + result.errors.join("\n// ") + "\n\n" + preview;
      return;
    }

    els.jsonPreview.textContent = preview;
  }

  function loadConfig(preferredHookId) {
    return request("/api/config").then(function (data) {
      state.files = normalizeArray(data && data.files).map(normalizeHookFile);
      state.readOnly = !!(data && data.readOnly);
      state.readOnlyReason = (data && data.readOnlyReason) || "";

      if (!state.files.length) {
        state.currentFile = "";
        state.currentHookId = "";
      } else {
        var fileExists = false;
        state.files.forEach(function (file) {
          if (file.path === state.currentFile) {
            fileExists = true;
          }
        });

        if (!fileExists) {
          state.currentFile = state.files[0].path;
        }

        var currentFile = getCurrentFile();
        var hooks = currentFile ? normalizeArray(currentFile.hooks) : [];
        var desiredHookId = preferredHookId || state.currentHookId;
        var hookExists = false;

        hooks.forEach(function (hookItem) {
          if (hookItem.id === desiredHookId) {
            hookExists = true;
          }
        });

        if (hookExists) {
          state.currentHookId = desiredHookId;
        } else {
          state.currentHookId = hooks.length ? hooks[0].id : "";
        }
      }

      showWorkspace();
      renderAll();
      setStatus(els.workspaceStatus, "配置已同步。", "success");
      loadUpdateStatus().catch(function () {
        renderUpdateStatus(null);
      });
    });
  }

  function renderUpdateStatus(data) {
    var status = data || {};
    els.currentVersion.textContent = status.currentVersion || "-";
    els.latestVersion.textContent = status.latestVersion || "尚未检查";
    els.checkUpdateBtn.disabled = status.enabled === false;

    if (status.error) {
      setStatus(els.updateStatus, status.error, "error");
    } else if (status.available) {
      setStatus(els.updateStatus, "发现新版本 " + status.latestVersion + "。请使用 CLI 完成更新。", "success");
    } else if (status.checkedAt) {
      setStatus(els.updateStatus, "当前已是最新版本。", "success");
    } else if (status.enabled === false) {
      setStatus(els.updateStatus, "更新检查已禁用。", "muted");
    } else {
      setStatus(els.updateStatus, "尚未检查更新。", "muted");
    }
  }

  function loadUpdateStatus() {
    return request("/api/update/status").then(function (data) {
      renderUpdateStatus(data);
    });
  }

  function checkForUpdate() {
    els.checkUpdateBtn.disabled = true;
    setStatus(els.updateStatus, "正在检查更新...", "muted");
    return request("/api/update/check", { method: "POST" }).then(function (data) {
      renderUpdateStatus(data);
    }).catch(function (error) {
      setStatus(els.updateStatus, error.message || "检查更新失败。", "error");
      loadUpdateStatus().catch(function () {});
    }).finally(function () {
      els.checkUpdateBtn.disabled = false;
    });
  }

  function handleSave() {
    if (!state.currentFile) {
      setStatus(els.workspaceStatus, "当前没有可写入的 hooks 文件。", "error");
      return;
    }

    var result = buildHookFromForm();
    if (result.errors.length) {
      setStatus(els.workspaceStatus, result.errors[0], "error");
      return;
    }

    var isUpdate = !!state.currentHookId;
    var requestPath = "/api/hooks";
    var method = isUpdate ? "PUT" : "POST";
    var payload = {
      file: state.currentFile,
      hook: result.hook
    };

    if (isUpdate) {
      payload.currentId = state.currentHookId;
    }

    request(requestPath, {
      method: method,
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload)
    }).then(function () {
      state.currentHookId = result.hook.id || "";
      return loadConfig(state.currentHookId);
    }).then(function () {
      setStatus(els.workspaceStatus, "保存成功。", "success");
    }).catch(function (error) {
      setStatus(els.workspaceStatus, error.message || "保存失败。", "error");
    });
  }

  function handleDelete() {
    if (!state.currentHookId) {
      setStatus(els.workspaceStatus, "当前没有选中的 hook。", "error");
      return;
    }

    if (!window.confirm("确认删除 hook “" + state.currentHookId + "” 吗？")) {
      return;
    }

    request("/api/hooks", {
      method: "DELETE",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        file: state.currentFile,
        currentId: state.currentHookId
      })
    }).then(function () {
      state.currentHookId = "";
      return loadConfig();
    }).then(function () {
      setStatus(els.workspaceStatus, "删除成功。", "success");
    }).catch(function (error) {
      setStatus(els.workspaceStatus, error.message || "删除失败。", "error");
    });
  }

  function bindBaseFormEvents() {
    [
      els.hookId,
      els.executeCommand,
      els.commandWorkingDirectory,
      els.commandTimeout,
      els.httpMethods,
      els.responseMessage,
      els.incomingPayloadContentType,
      els.successHttpResponseCode,
      els.triggerRuleMismatchHttpResponseCode,
      els.maxConcurrency
    ].forEach(function (input) {
      input.addEventListener("input", updatePreview);
    });

    [
      els.captureCommandOutput,
      els.captureCommandOutputOnError,
      els.triggerSignatureSoftFailures,
      els.keepFileEnvironment
    ].forEach(function (checkbox) {
      checkbox.addEventListener("change", updatePreview);
    });

    Object.keys(collectionDefs).forEach(function (key) {
      var def = collectionDefs[key];
      def.addButton.addEventListener("click", function () {
        appendCollectionRow(def, {});
        updatePreview();
      });
    });
  }

  els.loginForm.addEventListener("submit", function (event) {
    event.preventDefault();

    request("/api/auth/login", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ code: els.totpCode.value })
    }).then(function () {
      els.totpCode.value = "";
      return loadConfig();
    }).catch(function (error) {
      setStatus(els.loginStatus, error.message || "登录失败。", "error");
    });
  });

  els.fileSelect.addEventListener("change", function () {
    state.currentFile = els.fileSelect.value;
    var currentFile = getCurrentFile();
    var hooks = currentFile ? normalizeArray(currentFile.hooks) : [];
    state.currentHookId = hooks.length ? hooks[0].id : "";
    renderAll();
    setStatus(els.workspaceStatus, "已切换文件。", "muted");
  });

  els.refreshBtn.addEventListener("click", function () {
    loadConfig(state.currentHookId).catch(function (error) {
      setStatus(els.workspaceStatus, error.message || "刷新失败。", "error");
    });
  });

  els.checkUpdateBtn.addEventListener("click", checkForUpdate);

  els.newHookBtn.addEventListener("click", function () {
    state.currentHookId = "";
    renderHookList();
    renderCurrentHook();
    applyReadOnlyState();
    setStatus(els.workspaceStatus, "正在创建新 hook。", "muted");
  });

  els.saveBtn.addEventListener("click", handleSave);
  els.deleteBtn.addEventListener("click", handleDelete);

  els.logoutBtn.addEventListener("click", function () {
    request("/api/auth/logout", { method: "POST" }).finally(function () {
      showLogin("已退出管理员会话。");
    });
  });

  bindBaseFormEvents();

  loadConfig().catch(function () {
    showLogin("请输入 TOTP 动态码。");
  });

})();
