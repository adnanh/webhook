#!/bin/bash

set -o pipefail

# 默认读取当前工作目录下的 .env.update-web。
# 也可以通过 UPDATE_WEB_ENV_FILE 指定其他配置文件路径。
UPDATE_WEB_ENV_FILE="${UPDATE_WEB_ENV_FILE:-.env.update-web}"
if [ -f "${UPDATE_WEB_ENV_FILE}" ]; then
  case "${UPDATE_WEB_ENV_FILE}" in
    */*) . "${UPDATE_WEB_ENV_FILE}" ;;
    *) . "./${UPDATE_WEB_ENV_FILE}" ;;
  esac
fi

# ===== 发布配置默认值 =====

WEB_DEV_OSS_BASE_URI="${WEB_DEV_OSS_BASE_URI:-}"
WEB_DEV_ACCESS_BASE_URL="${WEB_DEV_ACCESS_BASE_URL:-}"
OSSUTIL_BIN="${OSSUTIL_BIN:-ossutil64}"
WEB_DEV_SUPER_CODE="${WEB_DEV_SUPER_CODE:-}"
WEB_DEV_ALLOWED_APPS="${WEB_DEV_ALLOWED_APPS:-}"

tar_warning_options=()
if tar --help 2>/dev/null | grep -q -- "--warning"; then
  tar_warning_options=(--warning=no-unknown-keyword)
fi

fail() {
  echo "$1"
  exit 0
}

require_env() {
  name="$1"
  value="${!name:-}"

  [ -n "${value}" ] || fail "缺少环境变量 ${name}，请填写实际值"
}

safe_name() {
  [[ "$1" =~ ^[A-Za-z0-9][A-Za-z0-9_-]{0,62}$ ]]
}

project_code_for() {
  app="$1"

  for allowed_app in ${WEB_DEV_ALLOWED_APPS}; do
    if [ "${allowed_app}" = "${app}" ]; then
      code_var="WEB_DEV_CODE_$(echo "${app}" | tr '[:lower:]-' '[:upper:]_')"
      echo "${!code_var:-}"
      return 0
    fi
  done

  return 1
}

validate_identity() {
  [ -z "${APP:-}" ] && fail "缺少 APP"
  [ -z "${CODE:-}" ] && fail "缺少 CODE"

  safe_name "${APP}" || fail "APP 不合法，只允许字母、数字、下划线和中划线，且不能以符号开头"

  if [ -n "${WEB_DEV_SUPER_CODE:-}" ] && [ "${CODE}" = "${WEB_DEV_SUPER_CODE}" ]; then
    return 0
  fi

  expected_code="$(project_code_for "${APP}")" || fail "APP 不在允许清单中"
  [ -z "${expected_code}" ] && fail "项目校验码未配置，请联系管理员"
  [ "${CODE}" = "${expected_code}" ] || fail "CODE 校验失败"
}

validate_tar_entries() {
  manifest_file="$1"

  awk '
    $0 ~ /^\/|(^|\/)\.\.(\/|$)/ {
      bad = 1
    }
    END {
      exit bad
    }
  ' "${manifest_file}"
}

pkg_file="${HOOK_FILE_PKG:-${HOOK_FILE_ATTACHMENT:-}}"
[ -z "${pkg_file}" ] && fail "NOT FOUND"
[ ! -f "${pkg_file}" ] && fail "NOT FOUND"

validate_identity

require_env WEB_DEV_OSS_BASE_URI
require_env WEB_DEV_ACCESS_BASE_URL

cache_dir="./.cache-web-dev-${APP}"
manifest="./.cache-web-dev-${APP}.manifest"

rm -rf -- "${cache_dir}"
rm -f -- "${manifest}"
mkdir -p "${cache_dir}" || fail "缓存目录创建失败"

tar "${tar_warning_options[@]}" -tzf "${pkg_file}" > "${manifest}" 2>/dev/null
if [ $? -ne 0 ]; then
  fail "资源格式不支持，请联系管理员"
fi

validate_tar_entries "${manifest}" || fail "资源包包含非法路径，请联系管理员"
[ ! -s "${manifest}" ] && fail "资源包为空，请联系管理员"

WEB_APP="dev-${APP}"
oss_target="${WEB_DEV_OSS_BASE_URI%/}/${WEB_APP}/"
access_url="${WEB_DEV_ACCESS_BASE_URL%/}/${WEB_APP}/"

update() {
  oss_output="$("${OSSUTIL_BIN}" sync "${cache_dir}" "${oss_target}" --delete -f 2>&1)"
  if [ $? -ne 0 ]; then
    echo "OSS 同步失败"
    echo "${oss_output}"
    return 1
  fi

  echo "资源同步完成"
  echo "对象储存地址: ${oss_target}"
  echo "访问地址: ${access_url}"
}

tar "${tar_warning_options[@]}" -xzf "${pkg_file}" -C "${cache_dir}" --no-same-owner --no-same-permissions 2>/dev/null
if [ $? -ne 0 ]; then
  fail "资源解压失败"
fi

find "${cache_dir}" -type d -name "__MACOSX" -prune -exec rm -rf {} +
find "${cache_dir}" -type f \( -name ".DS_Store" -o -name "._*" \) -delete

if find "${cache_dir}" -type l -print -quit | grep -q .; then
  fail "资源包包含符号链接，请联系管理员"
fi

if ! find "${cache_dir}" -mindepth 1 -print -quit | grep -q .; then
  fail "资源包为空，请联系管理员"
fi

update && echo "更新成功" && exit 0

echo "更新失败"
exit 0
