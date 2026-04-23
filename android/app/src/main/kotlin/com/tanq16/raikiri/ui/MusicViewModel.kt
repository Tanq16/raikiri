package com.tanq16.raikiri.ui

import androidx.lifecycle.ViewModel
import androidx.lifecycle.ViewModelProvider
import androidx.lifecycle.viewModelScope
import com.tanq16.raikiri.data.api.FileEntry
import com.tanq16.raikiri.data.repository.MusicRepository
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

class MusicViewModel(initialRepository: MusicRepository) : ViewModel() {

    var repository: MusicRepository = initialRepository
        private set

    sealed class UiState {
        data object Loading : UiState()
        data class Success(val items: List<FileEntry>) : UiState()
        data class Error(val message: String) : UiState()
    }

    private val _allSongsState = MutableStateFlow<UiState>(UiState.Loading)
    val allSongsState: StateFlow<UiState> = _allSongsState.asStateFlow()

    private val _folderState = MutableStateFlow<UiState>(UiState.Loading)
    val folderState: StateFlow<UiState> = _folderState.asStateFlow()

    private val _searchQuery = MutableStateFlow("")
    val searchQuery: StateFlow<String> = _searchQuery.asStateFlow()

    private val _searchResults = MutableStateFlow<List<FileEntry>>(emptyList())
    val searchResults: StateFlow<List<FileEntry>> = _searchResults.asStateFlow()

    val serverUrl: String get() = repository.serverUrl

    fun updateRepository(newRepo: MusicRepository) {
        repository = newRepo
        loadAllSongs()
    }

    fun loadAllSongs() {
        viewModelScope.launch {
            _allSongsState.value = UiState.Loading
            repository.getAllSongs()
                .onSuccess { _allSongsState.value = UiState.Success(it) }
                .onFailure { _allSongsState.value = UiState.Error(it.message ?: "Failed to load songs") }
        }
    }

    fun loadFolder(path: String) {
        viewModelScope.launch {
            _folderState.value = UiState.Loading
            repository.listFolder(path)
                .onSuccess { _folderState.value = UiState.Success(it) }
                .onFailure { _folderState.value = UiState.Error(it.message ?: "Failed to load folder") }
        }
    }

    fun setSearchQuery(query: String) {
        _searchQuery.value = query
        _searchResults.value = if (query.length >= 2) {
            repository.search(query)
        } else {
            emptyList()
        }
    }

    fun refresh() {
        repository.clearCache()
        loadAllSongs()
    }

    class Factory(private val initialRepository: MusicRepository) : ViewModelProvider.Factory {
        @Suppress("UNCHECKED_CAST")
        override fun <T : ViewModel> create(modelClass: Class<T>): T =
            MusicViewModel(initialRepository) as T
    }
}
