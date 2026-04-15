function normalizeDisplayName(value, fallback) {
  const text = String(value || '').trim()
  return text || fallback
}

function normalizeCharacterPov(character) {
  return character?.pov === 'second' ? 'second' : 'third'
}

export function resolveUserDisplayName(character, user) {
  if (normalizeCharacterPov(character) === 'second') {
    return '你'
  }

  if (character?.use_custom_user) {
    const customUserName = String(character?.user_name || '').trim()
    if (customUserName) return customUserName
  }

  return normalizeDisplayName(user?.user_name, '你')
}

export function resolveCharacterDisplayName(character) {
  return normalizeDisplayName(character?.name, '角色')
}

export function renderRolePlaceholders(text, { character, user } = {}) {
  const content = String(text || '')
  if (!content) return ''

  const charName = resolveCharacterDisplayName(character)
  const userName = resolveUserDisplayName(character, user)

  return content
    .replaceAll('{{char}}', charName)
    .replaceAll('{{Char}}', charName)
    .replaceAll('{{user}}', userName)
    .replaceAll('{{User}}', userName)
}
