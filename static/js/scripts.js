function toggleCompletion(id) {
  fetch(`/complete?id=${id}`, {
      method: 'POST',
  })
  .then(response => {
      if (response.redirected) {
          window.location.href = response.url;
      }
  })
  .catch(error => console.error('Error:', error));
}


function filterTasks(status) {
  const items = document.querySelectorAll('.list-group-item');
  items.forEach(item => {
      const checkbox = item.querySelector('input[type="checkbox"]');
      if (status === 'all') {
          item.style.display = 'flex';
      } else if (status === 'completed') {
          item.style.display = checkbox.checked ? 'flex' : 'none';
      } else if (status === 'pending') {
          item.style.display = !checkbox.checked ? 'flex' : 'none';
      }
  });
}
